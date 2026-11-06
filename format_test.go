// Copyright 2026 Patrick Scheid. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonduration

import (
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"math/big"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	maxD = time.Duration(math.MaxInt64)
	minD = time.Duration(math.MinInt64)
	d90  = 90*time.Minute + 500*time.Millisecond
)

func TestFormat(t *testing.T) {
	tests := []struct {
		f    Format
		d    time.Duration
		want string
	}{
		{Units, 0, "0s"},
		{Units, d90, "1h30m0.5s"},
		{Units, 1500 * time.Nanosecond, "1.5µs"},
		{Units, minD, "-2562047h47m16.854775808s"},

		{Sec, 0, "0"},
		{Sec, time.Nanosecond, "0.000000001"},
		{Sec, -time.Nanosecond, "-0.000000001"},
		{Sec, d90, "5400.5"},
		{Sec, maxD, "9223372036.854775807"},
		{Sec, minD, "-9223372036.854775808"},
		{Milli, d90, "5400500"},
		{Milli, 1500 * time.Microsecond, "1.5"},
		{Micro, d90, "5400500000"},
		{Nano, d90, "5400500000000"},
		{Nano, maxD, "9223372036854775807"},
		{Nano, minD, "-9223372036854775808"},

		{ISO8601, 0, "PT0S"},
		{ISO8601, time.Nanosecond, "PT0.000000001S"},
		{ISO8601, d90, "PT1H30M0.5S"},
		{ISO8601, time.Hour + time.Second, "PT1H1S"},
		{ISO8601, -time.Minute, "-PT1M"},
		{ISO8601, maxD, "PT2562047H47M16.854775807S"},
		{ISO8601, minD, "-PT2562047H47M16.854775808S"},

		{Base60, 0, "0:00:00"},
		{Base60, d90, "1:30:00.5"},
		{Base60, -time.Second, "-0:00:01"},
		{Base60, 100 * time.Hour, "100:00:00"},
		{Base60, maxD, "2562047:47:16.854775807"},
		{Base60, minD, "-2562047:47:16.854775808"},

		{ProtoJSON, 0, "0s"},
		{ProtoJSON, time.Nanosecond, "0.000000001s"},
		{ProtoJSON, 1500 * time.Millisecond, "1.500s"},
		{ProtoJSON, 1500 * time.Microsecond, "0.001500s"},
		{ProtoJSON, -1500 * time.Millisecond, "-1.500s"},
		{ProtoJSON, d90, "5400.500s"},
		{ProtoJSON, maxD, "9223372036.854775807s"},
		{ProtoJSON, minD, "-9223372036.854775808s"},
	}
	for _, tt := range tests {
		got := tt.f.Format(tt.d)
		if got != tt.want {
			t.Errorf("%s.Format(%d) = %q, want %q", tt.f, int64(tt.d), got, tt.want)
			continue
		}
		back, err := tt.f.Parse(got)
		if err != nil || back != tt.d {
			t.Errorf("%s.Parse(%q) = %v, %v; want %v", tt.f, got, back, err, tt.d)
		}
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		f       Format
		in      string
		want    time.Duration
		wantErr error // nil means success
	}{
		// Base-10.
		{Sec, "-0", 0, nil},
		{Sec, "1.5", 1500 * time.Millisecond, nil},
		{Sec, "0.0000000019", time.Nanosecond, nil}, // sub-nanosecond digits are truncated
		{Sec, "", 0, strconv.ErrSyntax},
		{Sec, "-", 0, strconv.ErrSyntax},
		{Sec, "+1", 0, strconv.ErrSyntax},
		{Sec, "01", 0, strconv.ErrSyntax},
		{Sec, "1.", 0, strconv.ErrSyntax},
		{Sec, ".5", 0, strconv.ErrSyntax},
		{Sec, "1e3", 0, strconv.ErrSyntax},
		{Sec, "1.5x", 0, strconv.ErrSyntax},
		{Sec, "9223372036.854775808", 0, strconv.ErrRange},
		{Sec, "-9223372036.854775809", 0, strconv.ErrRange},
		{Nano, "9223372036854775808", 0, strconv.ErrRange},
		{Nano, "99999999999999999999999", 0, strconv.ErrRange},
		{Nano, "1.5", 1, nil}, // fractions of the unit are truncated

		// ISO 8601.
		{ISO8601, "PT1H1S", time.Hour + time.Second, nil}, // invalid per RFC 3339 Appendix A, valid ISO 8601
		{ISO8601, "pt1h30m", 90 * time.Minute, nil},
		{ISO8601, "+PT1S", time.Second, nil},
		{ISO8601, "-PT1.5S", -1500 * time.Millisecond, nil},
		{ISO8601, "PT0,5S", 500 * time.Millisecond, nil},
		{ISO8601, "PT1.5H", d90 - 500*time.Millisecond, nil},
		{ISO8601, "PT007S", 7 * time.Second, nil},
		{ISO8601, "-PT0S", 0, nil},
		{ISO8601, "P1D", 0, ErrNominalUnits},
		{ISO8601, "P1W", 0, ErrNominalUnits},
		{ISO8601, "P1Y2M", 0, ErrNominalUnits},
		{ISO8601, "P1DT1H", 0, ErrNominalUnits},
		{ISO8601, "", 0, strconv.ErrSyntax},
		{ISO8601, "P", 0, strconv.ErrSyntax},
		{ISO8601, "PT", 0, strconv.ErrSyntax},
		{ISO8601, "T1S", 0, strconv.ErrSyntax},
		{ISO8601, "PT-1S", 0, strconv.ErrSyntax},
		{ISO8601, "PT1S1M", 0, strconv.ErrSyntax},
		{ISO8601, "PT1.5H30M", 0, strconv.ErrSyntax},
		{ISO8601, "PT1.S", 0, strconv.ErrSyntax},
		{ISO8601, "PT1.5e1H", 0, strconv.ErrSyntax},
		{ISO8601, "P1.5D", 0, strconv.ErrSyntax},
		{ISO8601, "PT1X", 0, strconv.ErrSyntax},
		{ISO8601, "PT2562048H", 0, strconv.ErrRange},
		{ISO8601, "-PT2562047H47M16.854775809S", 0, strconv.ErrRange},

		// Base-60.
		{Base60, "0:00:00.000000001", time.Nanosecond, nil},
		{Base60, "-1:00:00", -time.Hour, nil},
		{Base60, "01:00:00", 0, strconv.ErrSyntax},
		{Base60, "1:5:00", 0, strconv.ErrSyntax},
		{Base60, "1:60:00", 0, strconv.ErrSyntax},
		{Base60, "1:00:60", 0, strconv.ErrSyntax},
		{Base60, "1:00", 0, strconv.ErrSyntax},
		{Base60, "1:00:00.", 0, strconv.ErrSyntax},
		{Base60, "+1:00:00", 0, strconv.ErrSyntax},
		{Base60, "2562048:00:00", 0, strconv.ErrRange},

		// Protobuf JSON.
		{ProtoJSON, "1s", time.Second, nil},
		{ProtoJSON, "1.5s", 1500 * time.Millisecond, nil},
		{ProtoJSON, "-0.000000001s", -time.Nanosecond, nil},
		{ProtoJSON, "1", 0, strconv.ErrSyntax},
		{ProtoJSON, "1S", 0, strconv.ErrSyntax},
		{ProtoJSON, "s", 0, strconv.ErrSyntax},
		{ProtoJSON, "1.0000000001s", 0, strconv.ErrSyntax},
		{ProtoJSON, "+1s", 0, strconv.ErrSyntax},
		{ProtoJSON, "9223372037s", 0, strconv.ErrRange},

		// Units: the grammar of time.ParseDuration, with exact fractions.
		{Units, "1h30m", 90 * time.Minute, nil},
		{Units, "1us", time.Microsecond, nil},
		{Units, "1µs", time.Microsecond, nil}, // micro sign
		{Units, "1μs", time.Microsecond, nil}, // Greek mu
		{Units, "+1.5h", 90 * time.Minute, nil},
		{Units, ".5s", 500 * time.Millisecond, nil},
		{Units, "5.s", 5 * time.Second, nil},
		{Units, "01s", time.Second, nil},
		{Units, "1h.5m", time.Hour + 30*time.Second, nil},
		{Units, "0", 0, nil},
		{Units, "-0", 0, nil},
		{Units, "0.0000000019s", time.Nanosecond, nil}, // truncated
		{Units, "0.0000000000002h", 0, nil},            // 0.72ns, truncated
		{Units, "-9223372036854775808ns", minD, nil},
		{Units, "", 0, strconv.ErrSyntax},
		{Units, "-", 0, strconv.ErrSyntax},
		{Units, "1", 0, strconv.ErrSyntax}, // missing unit
		{Units, "1x", 0, strconv.ErrSyntax},
		{Units, ".s", 0, strconv.ErrSyntax},
		{Units, " 1s", 0, strconv.ErrSyntax},
		{Units, "1h-1m", 0, strconv.ErrSyntax},
		{Units, "PT1H", 0, strconv.ErrSyntax},
		{Units, "2562048h", 0, strconv.ErrRange},
		{Units, "9223372036854775808ns", 0, strconv.ErrRange},
		{Units, "99999999999999999999999ns", 0, strconv.ErrRange},
		{Units, "2562047h47m16.854775807s1ns", 0, strconv.ErrRange},
	}
	for _, tt := range tests {
		got, err := tt.f.Parse(tt.in)
		switch {
		case tt.wantErr == nil && err != nil:
			t.Errorf("%s.Parse(%q) error: %v", tt.f, tt.in, err)
		case tt.wantErr != nil && !errors.Is(err, tt.wantErr):
			t.Errorf("%s.Parse(%q) error = %v, want %v", tt.f, tt.in, err, tt.wantErr)
		case tt.wantErr == nil && got != tt.want:
			t.Errorf("%s.Parse(%q) = %v, want %v", tt.f, tt.in, got, tt.want)
		}
	}
	// Units errors say what is wrong, like those of time.ParseDuration.
	for in, want := range map[string]string{
		"1":  `invalid duration "1": missing unit: invalid syntax`,
		"1x": `invalid duration "1x": unknown unit "x": invalid syntax`,
	} {
		if _, err := Units.Parse(in); err == nil || err.Error() != want {
			t.Errorf("Units.Parse(%q) error = %v, want %s", in, err, want)
		}
	}
}

// FuzzUnitsMatchesParseDuration checks that Units accepts exactly what
// time.ParseDuration accepts, with the same value. time.ParseDuration
// computes each fraction through float64, so each component with a
// fraction may come out 1ns apart, which can also move a value just past
// the range of a time.Duration.
func FuzzUnitsMatchesParseDuration(f *testing.F) {
	for _, s := range []string{
		"1h30m", "1.5us", "1µs", "1μs", "0", "-0", "+.5s", "5.s", "1h1h", "0.1h0.1h0.1h",
		"9223372036854775807ns", "-9223372036854775808ns", "9223372036854775808ns",
		"2562047h47m16.854775807s", "2562047h47m16.854775808s", "-2562047h47m16.854775808s",
		"1.0000000000000000000001s", "0.123456789012345h", ".s", "1", "", "1x", "1h-1m",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		want, wantErr := time.ParseDuration(s)
		got, err := Units.Parse(s)
		slack := uint64(strings.Count(s, "."))
		near := func(a, b time.Duration) bool { // in uint64, so no difference wraps
			if a < b {
				a, b = b, a
			}
			return uint64(a)-uint64(b) <= slack
		}
		nearLimit := func(d time.Duration) bool { return near(d, maxD) || near(d, minD) }
		switch {
		case err == nil && wantErr == nil && !near(got, want):
			t.Fatalf("Parse(%q) = %d, time.ParseDuration = %d", s, int64(got), int64(want))
		case err == nil && wantErr != nil && !nearLimit(got):
			t.Fatalf("Parse(%q) = %d, but time.ParseDuration fails: %v", s, int64(got), wantErr)
		case err != nil && wantErr == nil && !nearLimit(want):
			t.Fatalf("Parse(%q) fails: %v, but time.ParseDuration = %d", s, err, int64(want))
		}
		if err != nil && !errors.Is(err, strconv.ErrSyntax) && !errors.Is(err, strconv.ErrRange) {
			t.Fatalf("Parse(%q) error %v wraps neither ErrSyntax nor ErrRange", s, err)
		}
	})
}

func TestFormatText(t *testing.T) {
	for _, f := range Formats() {
		b, err := f.MarshalText()
		if err != nil || string(b) != string(f) {
			t.Errorf("%s.MarshalText = %s, %v", f, b, err)
		}
		var g Format
		if err := g.UnmarshalText(b); err != nil || g != f {
			t.Errorf("UnmarshalText(%q) = %q, %v; want %q", b, g, err, f)
		}
	}

	// Names match exactly, so other spellings are rejected, and the error
	// lists the names. A failed decoding leaves the format unchanged.
	const names = "units, sec, milli, micro, nano, iso8601, base60, protojson"
	for _, in := range []string{"bogus", "ISO8601", "Nano", " sec", ""} {
		g := Sec
		err := g.UnmarshalText([]byte(in))
		if want := fmt.Sprintf("jsonduration: unknown format %q, want one of %s", in, names); err == nil || err.Error() != want || g != Sec {
			t.Errorf("UnmarshalText(%q) = %q, %v; want %q and %s", in, g, err, Sec, want)
		}
	}
	if _, err := Format("bogus").MarshalText(); err == nil {
		t.Error("MarshalText of an invalid format succeeded")
	}

	// Formats can come from flags and from JSON configuration.
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var fromFlag Format
	fs.TextVar(&fromFlag, "format", ISO8601, "duration format")
	if err := fs.Parse([]string{"-format=nano"}); err != nil || fromFlag != Nano {
		t.Errorf("flag nano: %q, %v; want nano", fromFlag, err)
	}
	if err := fs.Parse([]string{"-format=Nano"}); err == nil {
		t.Error("flag Nano succeeded, want an error")
	}
	var cfg struct{ Format Format }
	if err := json.Unmarshal([]byte(`{"Format":"iso8601"}`), &cfg); err != nil || cfg.Format != ISO8601 {
		t.Errorf("JSON iso8601: %q, %v; want iso8601", cfg.Format, err)
	}
	for _, in := range []string{`{"Format":"ISO8601"}`, `{"Format":"weeks"}`} {
		if err := json.Unmarshal([]byte(in), &cfg); err == nil {
			t.Errorf("JSON %s succeeded, want an error", in)
		}
	}
}

// TestFormatValidExact: a converted string is only valid in the exact
// spelling of a format, and only then equal to the constant.
func TestFormatValidExact(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"iso8601", true},
		{"ISO8601", false},
		{"Iso8601", false},
		{"iso8601 ", false}, // a trailing space
	}
	for _, tt := range tests {
		f := Format(tt.in)
		if f.Valid() != tt.want || (f == ISO8601) != tt.want {
			t.Errorf("Format(%q): Valid = %v, == ISO8601 is %v; want both %v", tt.in, f.Valid(), f == ISO8601, tt.want)
		}
	}
}

func FuzzRoundTrip(f *testing.F) {
	for _, d := range []int64{0, 1, -1, int64(d90), math.MaxInt64, math.MinInt64, 1500} {
		f.Add(d)
	}
	f.Fuzz(func(t *testing.T, n int64) {
		d := time.Duration(n)
		for _, fm := range Formats() {
			s := fm.Format(d)
			got, err := fm.Parse(s)
			if err != nil || got != d {
				t.Fatalf("%s: Parse(Format(%d)) = %q -> %d, %v", fm, n, s, int64(got), err)
			}
		}
	})
}

func FuzzParse(f *testing.F) {
	for _, s := range []string{
		"1h30m", "5400.5", "PT1H30M0.5S", "1:30:00.5", "1.500s", "-0", "P1D", "PT1.5H",
		// Texts that more than one format accepts.
		"0", "+0", "1.5s", ".5s", "1.000000001s", "0.3s", "-0.000000001s",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		accepted := map[Format]time.Duration{}
		for _, fm := range Formats() {
			d, err := fm.Parse(s) // must not panic
			if err != nil {
				continue
			}
			accepted[fm] = d
			// Anything accepted must re-encode to something that parses to the same value.
			again, err := fm.Parse(fm.Format(d))
			if err != nil || again != d {
				t.Fatalf("%s: %q -> %d -> %q -> %d, %v", fm, s, int64(d), fm.Format(d), int64(again), err)
			}
		}
		// AlsoAccept relies on this: formats that may be accepted together
		// (any two string formats, or a string format and one numeric
		// format) never read the same text as different durations.
		for a, da := range accepted {
			for b, db := range accepted {
				if a < b && (!a.IsNumeric() || !b.IsNumeric()) && da != db {
					t.Fatalf("%q is %d in %s but %d in %s", s, int64(da), a, int64(db), b)
				}
			}
		}
	})
}

// TestISO8601FractionExact checks fractions of hours and minutes against
// exact rational arithmetic: the result must be truncated, never rounded,
// however far away the digit that decides it is.
func TestISO8601FractionExact(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"PT0.0000000000002H", 0},                          // 0.72ns
		{"PT0.00000000002M", 1},                            // 1.2ns
		{"PT0.02777777777777777777H", 100*time.Second - 1}, // just below 1/36 h
		{"PT0.02777777777777777778H", 100 * time.Second},   // just above 1/36 h
		{"PT0.16666666666666666666M", 10*time.Second - 1},  // just below 1/6 min
		{"PT0.16666666666666666667M", 10 * time.Second},    // just above 1/6 min
		{"-PT0.0000000000002H", 0},
		{"-PT0.00000000002M", -1},
	}
	for _, tt := range tests {
		got, err := ISO8601.Parse(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("Parse(%q) = %d, %v; want %d", tt.in, int64(got), err, int64(tt.want))
		}
	}

	r := rand.New(rand.NewPCG(1, 2))
	for range 20000 {
		digits := make([]byte, 1+r.IntN(25))
		for i := range digits {
			digits[i] = byte('0' + r.IntN(10))
		}
		for _, u := range []struct {
			des  string
			unit time.Duration
		}{{"H", time.Hour}, {"M", time.Minute}, {"S", time.Second}} {
			in := "PT0." + string(digits) + u.des
			exact, _ := new(big.Rat).SetString("0." + string(digits))
			exact.Mul(exact, new(big.Rat).SetInt64(int64(u.unit)))
			want := new(big.Int).Quo(exact.Num(), exact.Denom()).Int64()
			if got, err := ISO8601.Parse(in); err != nil || int64(got) != want {
				t.Fatalf("Parse(%q) = %d, %v; want %d", in, int64(got), err, want)
			}
		}
	}
}

func TestValid(t *testing.T) {
	for _, f := range Formats() {
		if !f.Valid() {
			t.Errorf("%q.Valid() = false", f)
		}
	}
	for _, f := range []Format{"", "bogus", "ISO8601", "Nano"} {
		if f.Valid() {
			t.Errorf("%q.Valid() = true", f)
		}
		if _, err := f.Parse("1"); err == nil {
			t.Errorf("%q.Parse succeeded, want error", f)
		}
	}
}

func TestInvalidFormatPanics(t *testing.T) {
	const bogus = Format("bogus")
	tests := map[string]struct {
		fn   func()
		want string // in the panic message
	}{
		"With":                {func() { With(bogus) }, "invalid format"},
		"Marshalers":          {func() { Marshalers(bogus) }, "invalid format"},
		"Unmarshalers":        {func() { Unmarshalers(bogus) }, "invalid format"},
		"Format":              {func() { bogus.Format(time.Second) }, "invalid format"},
		"Append":              {func() { bogus.Append(nil, time.Second) }, "invalid format"},
		"AlsoAccept invalid":  {func() { With(ISO8601, AlsoAccept(bogus)) }, "invalid format"},
		"AlsoAccept numeric":  {func() { With(Nano, AlsoAccept(Sec)) }, "both nano and sec"},
		"AlsoAccept numerics": {func() { Unmarshalers(ISO8601, AlsoAccept(Milli, Micro)) }, "both milli and micro"},
	}
	for name, tt := range tests {
		func() {
			defer func() {
				msg, _ := recover().(string)
				if !strings.Contains(msg, tt.want) {
					t.Errorf("%s: panic %q, want one containing %q", name, msg, tt.want)
				}
			}()
			tt.fn()
		}()
	}
}
