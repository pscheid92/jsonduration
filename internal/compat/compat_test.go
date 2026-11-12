package compat

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	jsonexp "github.com/go-json-experiment/json"
	"github.com/pscheid92/jsonduration"
)

// formatTag turns the removed `format` tag option back on.
var formatTag = jsonexp.ExperimentalSupportFormatTag(true)

// A target is a struct with one duration field, D, read and written once with the tag option and once with
// jsonduration.
type target struct {
	name   string
	format jsonduration.Format
	tagged reflect.Type // D has the tag option of format, as in `json:"d,format:sec"`.
	plain  reflect.Type // D has no tag option, so jsonduration.With(format) decides.
}

// targets returns a target for every format, with and without the ,string option.
// The tag option rejects ,string on a string format, where jsonduration ignores it, as TestStringOptionOnStringFormats
// shows, so there the target compares with the tag option without ,string.
func targets() []target {
	all := make([]target, 0, 2*len(jsonduration.Formats()))
	for _, f := range jsonduration.Formats() {
		for _, stringOption := range []string{"", ",string"} {
			tagStringOption := stringOption
			if !isNumeric(f) {
				tagStringOption = ""
			}

			tagged := structWithTag(`json:"d` + tagStringOption + `,format:` + string(f) + `"`)
			plain := structWithTag(`json:"d` + stringOption + `"`)
			all = append(all, target{string(f) + stringOption, f, tagged, plain})
		}
	}

	return all
}

func isNumeric(f jsonduration.Format) bool {
	return f != jsonduration.Units && f != jsonduration.ISO8601
}

func structWithTag(tag string) reflect.Type {
	field := reflect.StructField{Name: "D", Type: reflect.TypeFor[time.Duration](), Tag: reflect.StructTag(tag)}
	return reflect.StructOf([]reflect.StructField{field})
}

func (tg target) marshalTagged(d time.Duration) ([]byte, error) {
	v := newStruct(tg.tagged, d)
	return json.Marshal(v.Interface(), formatTag)
}

func (tg target) marshalPlain(d time.Duration) ([]byte, error) {
	v := newStruct(tg.plain, d)
	return json.Marshal(v.Interface(), jsonduration.With(tg.format))
}

func (tg target) unmarshalTagged(b []byte) (time.Duration, error) {
	return unmarshalInto(tg.tagged, b, formatTag)
}

func (tg target) unmarshalPlain(b []byte) (time.Duration, error) {
	return unmarshalInto(tg.plain, b, jsonduration.With(tg.format))
}

func newStruct(t reflect.Type, d time.Duration) reflect.Value {
	v := reflect.New(t).Elem()
	v.Field(0).SetInt(int64(d))

	return v
}

// unmarshalInto decodes b into a new struct of type t and returns D, or 0 with the error.
func unmarshalInto(t reflect.Type, b []byte, opts json.Options) (time.Duration, error) {
	v := reflect.New(t)
	if err := json.Unmarshal(b, v.Interface(), opts); err != nil {
		return 0, err
	}

	d := time.Duration(v.Elem().Field(0).Int())
	return d, nil
}

func TestMarshalMatchesFormatTag(t *testing.T) {
	durations := []time.Duration{0, 1, -1, 1500 * time.Millisecond, 90 * time.Minute, math.MaxInt64, math.MinInt64}

	for _, tg := range targets() {
		for _, d := range durations {
			checkMarshal(t, tg, d)
		}
	}
}

func FuzzMarshalMatchesFormatTag(f *testing.F) {
	seeds := []int64{0, 1, -1, int64(1500 * time.Millisecond), int64(90 * time.Minute), math.MaxInt64, math.MinInt64}
	for _, n := range seeds {
		f.Add(n)
	}

	targets := targets()
	f.Fuzz(func(t *testing.T, n int64) {
		for _, tg := range targets {
			checkMarshal(t, tg, time.Duration(n))
		}
	})
}

func checkMarshal(t *testing.T, tg target, d time.Duration) {
	t.Helper()

	want, wantErr := tg.marshalTagged(d)
	got, err := tg.marshalPlain(d)
	if err != nil || wantErr != nil || !bytes.Equal(got, want) {
		t.Fatalf("%s: Marshal(%d) = %s, %v; the tag option writes %s, %v", tg.name, d, got, err, want, wantErr)
	}
}

// FuzzUnmarshalMatchesFormatTag checks that every JSON value decodes to the same duration as with the tag option,
// or fails with both, except where jsonduration intends to differ; see TestDifferencesFromFormatTag.
// Where both fail, the errors may differ: jsonduration names the cause.
func FuzzUnmarshalMatchesFormatTag(f *testing.F) {
	seeds := []string{
		`"1h30m"`, `"-1.5s"`, `"+.5h"`, `"1.s"`, `"PT1H30M"`, `"-PT0.5S"`, `"PT0.99999999999999999999H"`, `"PT1.e5H"`,
		`"P1D"`, `"P1Y2M"`, `1.5`, `-0`, `-0.0000000001`, `9223372036.854775807`, `"1.5"`, `null`, `true`, `1e3`,
		`".9007199279999999s"`, `"P0D"`, `"P0Y0M0DT1H30M0S"`, `"P0Y1D"`, `"-PT0,0000000000002H"`,
	}
	for _, s := range seeds {
		f.Add(s)
	}

	targets := targets()
	f.Fuzz(func(t *testing.T, value string) {
		if !jsontext.Value(value).IsValid() {
			return
		}

		in := []byte(`{"d":` + value + `}`)

		for _, tg := range targets {
			want, wantErr := tg.unmarshalTagged(in)
			got, err := tg.unmarshalPlain(in)

			agree := (err == nil) == (wantErr == nil) && got == want
			if !agree && !intendedDifference(tg.format, value, want, wantErr, got, err) {
				t.Fatalf("%s: Unmarshal(%s) = %v, %v; the tag option reads %v, %v",
					tg.name, in, got, err, want, wantErr)
			}
		}
	})
}

var (
	// A negative number that truncates to zero, such as -0 or -0.0000000001, possibly quoted.
	negativeZero = regexp.MustCompile(`^"?-0(\.0*\d*)?"?$`)

	// A fraction that starts with e or E, as in "PT1.e5H".
	fractionStartingWithE = regexp.MustCompile(`[.,][eE]`)

	// Calendar units with a count of zero and nothing else before the clock components, as in "P0D" or "P0Y0M0DT1H".
	onlyZeroCalendarUnits = regexp.MustCompile(`(?i)^"[+-]?P(0+[YMWD])+(T.*)?"$`)

	// A fraction of hours or minutes as the last component, as in "PT1.5H".
	fractionOfHoursOrMinutes = regexp.MustCompile(`[.,]\d+[HhMm]"$`)
)

// intendedDifference reports whether jsonduration differs from the tag option on value in one of the ways that
// TestDifferencesFromFormatTag lists.
func intendedDifference(f jsonduration.Format, value string, want time.Duration, wantErr error,
	got time.Duration, err error) bool {
	value = strings.Trim(value, " \t\r\n")

	switch {
	case f == jsonduration.Units:
		return roundsFractionsThroughFloat(value, want, wantErr, got, err)
	case isNumeric(f) && errors.Is(wantErr, strconv.ErrRange) && err == nil && got == 0:
		return negativeZero.MatchString(value)
	case f == jsonduration.ISO8601 && wantErr == nil && errors.Is(err, strconv.ErrSyntax):
		return fractionStartingWithE.MatchString(value)
	case f == jsonduration.ISO8601 && wantErr == nil && err == nil && oneFurtherFromZero(want, got):
		return fractionOfHoursOrMinutes.MatchString(value)
	case f == jsonduration.ISO8601 && wantErr != nil && err == nil:
		return onlyZeroCalendarUnits.MatchString(value)
	}

	return false
}

// oneFurtherFromZero reports whether want is a nanosecond further from zero than got, on the same side of it:
// where the tag option rounds a fraction up and jsonduration truncates it, for a negative duration as for a positive.
func oneFurtherFromZero(want, got time.Duration) bool {
	if want < 0 {
		return got-want == 1
	}

	return want-got == 1
}

// roundsFractionsThroughFloat reports whether the results for Units differ only as far as time.ParseDuration, which
// the tag option uses, rounds fractions through float64: by at most a nanosecond for each fraction, which at the end of
// the range can also decide whether a value is in range at all.
func roundsFractionsThroughFloat(value string, want time.Duration, wantErr error, got time.Duration, err error) bool {
	var s string
	if json.Unmarshal([]byte(value), &s) != nil {
		return false
	}

	slack := uint64(strings.Count(s, "."))
	near := func(a, b time.Duration) bool {
		if a < b {
			a, b = b, a
		}

		return uint64(a)-uint64(b) <= slack
	}

	nearLimit := func(d time.Duration) bool {
		return near(d, math.MaxInt64) || near(d, math.MinInt64)
	}

	switch {
	case err == nil && wantErr == nil:
		return near(got, want)
	case err == nil:
		return nearLimit(got)
	case wantErr == nil:
		return nearLimit(want) && errors.Is(err, strconv.ErrRange)
	}

	return false
}

// TestDifferencesFromFormatTag lists how jsonduration decodes differently from the tag option, and why. Each row
// checks both sides, so a row fails when a fix in encoding/json/v2 makes it obsolete.
func TestDifferencesFromFormatTag(t *testing.T) {
	tests := []struct {
		format jsonduration.Format
		value  string
		tag    result
		ours   result
		why    string
	}{
		{
			jsonduration.Sec, `-0`, rangeError, zero,
			"encoding/json/v2 reports -0 as out of range",
		},
		{
			jsonduration.Nano, `-0`, rangeError, zero,
			"encoding/json/v2 reports -0 as out of range",
		},
		{
			jsonduration.Milli, `-0.0000000001`, rangeError, zero,
			"encoding/json/v2 reports what truncates to -0 as out of range",
		},
		{
			jsonduration.ISO8601, `"PT1.e5H"`, value(time.Hour), syntaxError,
			"encoding/json/v2 does not check the first character of a fraction",
		},
		{
			jsonduration.ISO8601, `"PT0.99999999999999999999H"`, value(time.Hour), value(time.Hour - 1),
			"jsonduration truncates exactly, encoding/json/v2 rounds through float64",
		},
		{
			jsonduration.ISO8601, `"PT0.0000000000002H"`, value(1), zero,
			"jsonduration truncates exactly, encoding/json/v2 rounds through float64",
		},
		{
			jsonduration.ISO8601, `"-PT0,0000000000002H"`, value(-1), zero,
			"jsonduration truncates toward zero, encoding/json/v2 rounds through float64, also when negative",
		},
		{
			jsonduration.Units, `".9007199279999999s"`, value(900719928), value(900719927),
			"jsonduration truncates exactly, time.ParseDuration rounds through float64",
		},
		{
			jsonduration.ISO8601, `"P0D"`, failure, zero,
			"zero days are no time at all, and some producers write zero this way",
		},
		{
			jsonduration.ISO8601, `"P0Y0M0DT1H30M0S"`, failure, value(90 * time.Minute),
			"some producers write every component, and a count of zero is no time at all",
		},
		{
			jsonduration.ISO8601, `"P1D"`, failure, calendarError,
			"both reject calendar units, but only jsonduration names the cause",
		},
		{
			jsonduration.Units, `"1x"`, failure, syntaxError,
			"both reject bad syntax, but only jsonduration names the cause",
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.format)+" "+tt.value, func(t *testing.T) {
			in := []byte(`{"d":` + tt.value + `}`)
			tg := target{format: tt.format, tagged: structWithTag(`json:"d,format:` + string(tt.format) + `"`),
				plain: structWithTag(`json:"d"`)}

			want, wantErr := tg.unmarshalTagged(in)
			if !tt.tag.matches(want, wantErr) {
				t.Errorf("the tag option reads %v, %v, want %s (%s)", want, wantErr, tt.tag, tt.why)
			}

			got, err := tg.unmarshalPlain(in)
			if !tt.ours.matches(got, err) {
				t.Errorf("jsonduration reads %v, %v, want %s (%s)", got, err, tt.ours, tt.why)
			}
		})
	}
}

// TestStringOptionOnStringFormats shows the one difference in encoding: the tag option rejects the ,string option on a
// string format, but jsonduration ignores it, since it cannot tell the option from json.StringifyNumbers, which must
// not break string formats.
func TestStringOptionOnStringFormats(t *testing.T) {
	withString := structWithTag(`json:"d,string"`)
	in := []byte(`{"d":"PT1H"}`)

	for _, f := range []jsonduration.Format{jsonduration.Units, jsonduration.ISO8601} {
		t.Run(string(f), func(t *testing.T) {
			tagged := structWithTag(`json:"d,string,format:` + string(f) + `"`)
			tg := target{string(f), f, tagged, withString}

			if _, err := tg.marshalTagged(time.Hour); err == nil {
				t.Error("the tag option writes ,string on a string format, want an error")
			}

			got, err := tg.marshalPlain(time.Hour)
			want := `{"d":"` + f.Format(time.Hour) + `"}`
			if err != nil || string(got) != want {
				t.Errorf("Marshal = %s, %v, want %s", got, err, want)
			}

			if _, err := tg.unmarshalTagged(in); err == nil {
				t.Error("the tag option reads ,string on a string format, want an error")
			}
		})
	}
}

// A result is what decoding should return: a duration, or an error that wraps cause.
// A failure without a cause matches any error, but only one that names none of jsonduration's causes.
type result struct {
	d     time.Duration
	fails bool
	cause error
}

var (
	zero          = value(0)
	failure       = result{fails: true}
	syntaxError   = result{fails: true, cause: strconv.ErrSyntax}
	rangeError    = result{fails: true, cause: strconv.ErrRange}
	calendarError = result{fails: true, cause: jsonduration.ErrCalendarUnits}
)

func value(d time.Duration) result {
	return result{d: d}
}

func (r result) matches(d time.Duration, err error) bool {
	switch {
	case !r.fails:
		return err == nil && d == r.d
	case r.cause != nil:
		return errors.Is(err, r.cause)
	}

	namesCause := errors.Is(err, strconv.ErrSyntax) || errors.Is(err, jsonduration.ErrCalendarUnits)
	return err != nil && !namesCause
}

func (r result) String() string {
	switch {
	case !r.fails:
		return r.d.String()
	case r.cause != nil:
		return "an error with " + r.cause.Error()
	}

	return "an error without a cause"
}
