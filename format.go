// Copyright 2026 Patrick Scheid. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// The base-10, ISO 8601 and base-60 codecs are adapted from
// encoding/json/v2/arshal_time.go (Go 1.27.3) and from the base60 support
// that github.com/go-json-experiment/json removed in commit d3c622f1b8.
// Those portions are Copyright 2020 The Go Authors; see LICENSE.

package jsonduration

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"strconv"
	"strings"
	"time"
)

// Format names a JSON representation of a [time.Duration]. Its value is
// the name of the format, such as "iso8601", and names match exactly, case
// included, as they did in the `format` struct tag option of
// encoding/json/v2.
//
// A Format converted from a string, as in Format(s), is not checked. Check
// such a value with [Format.Valid] before passing it to [With], which
// panics on an invalid format. [Format.UnmarshalText] checks for itself.
type Format string

const (
	// Units is a JSON string produced by [time.Duration.String], e.g.
	// "1h30m0.5s". Like the standard library, durations under one
	// millisecond use "µs". Input is what [time.ParseDuration] accepts,
	// such as "1h30m" or "1.5us", but fractions are computed exactly
	// instead of through float64, and errors wrap [strconv.ErrSyntax] or
	// [strconv.ErrRange] like those of the other formats.
	Units Format = "units"

	// Sec is a JSON number of seconds with up to 9 fractional digits,
	// e.g. 5400.5. It is exact: the value is never routed through float64.
	Sec Format = "sec"

	// Milli is a JSON number of milliseconds, e.g. 5400500.
	Milli Format = "milli"

	// Micro is a JSON number of microseconds, e.g. 5400500000.
	Micro Format = "micro"

	// Nano is a JSON integer of nanoseconds, e.g. 5400500000000.
	// This is the representation of encoding/json (v1).
	Nano Format = "nano"

	// ISO8601 is a JSON string with the restricted ISO 8601 profile of
	// encoding/json/v2, e.g. "PT1H30M0.5S". Output only uses the accurate
	// hour, minute and second designators. Input follows the grammar of
	// JavaScript's Temporal.Duration; unlike the standard library, input
	// with the nominal designators year, month, week or day is rejected,
	// unless [AcceptDays] allows days and weeks.
	ISO8601 Format = "iso8601"

	// Base60 is a JSON string in H:MM:SS.SSSSSSSSS notation, e.g. "1:30:00.5".
	// It was part of the original encoding/json/v2 proposal and was removed
	// after the vote in https://go.dev/issue/71631.
	Base60 Format = "base60"

	// ProtoJSON is the JSON string of google.protobuf.Duration:
	// seconds followed by "s", e.g. "5400.500s". Output has 0, 3, 6 or 9
	// fractional digits as the protobuf JSON mapping requires; input
	// accepts 0 to 9 fractional digits.
	ProtoJSON Format = "protojson"
)

// Formats returns every supported format, in a new slice on each call.
func Formats() []Format {
	return []Format{Units, Sec, Milli, Micro, Nano, ISO8601, Base60, ProtoJSON}
}

// Valid reports whether f is exactly one of the supported formats, so
// that, for example, "ISO8601" is not valid. Use it to check a Format
// converted from a string before passing it to [With].
func (f Format) Valid() bool {
	switch f {
	case Units, Sec, Milli, Micro, Nano, ISO8601, Base60, ProtoJSON:
		return true
	}
	return false
}

// IsNumeric reports whether f encodes as a JSON number.
// Numeric formats are written as JSON strings when used as an object name
// or when json.StringifyNumbers is set.
func (f Format) IsNumeric() bool {
	return f.pow10() != 0
}

func (f Format) pow10() uint64 {
	switch f {
	case Sec:
		return 1e9
	case Milli:
		return 1e6
	case Micro:
		return 1e3
	case Nano:
		return 1
	}
	return 0
}

// Append appends the representation of d in format f to b,
// without JSON quoting. It panics if f is not valid.
func (f Format) Append(b []byte, d time.Duration) []byte {
	switch f {
	case Units:
		return append(b, d.String()...)
	case Sec, Milli, Micro, Nano:
		return appendDurationBase10(b, d, f.pow10())
	case ISO8601:
		return appendDurationISO8601(b, d)
	case Base60:
		return appendDurationBase60(b, d)
	case ProtoJSON:
		return appendDurationProto(b, d)
	}
	panic(fmt.Sprintf("jsonduration: invalid format %q", string(f)))
}

// Format returns the representation of d in format f, without JSON quoting.
// It panics if f is not valid.
func (f Format) Format(d time.Duration) string {
	return string(f.Append(nil, d))
}

// Parse parses s, which must be in format f and not JSON quoted.
// Only format f is accepted; there is no fallback to other formats.
// Fractional digits below one nanosecond are truncated toward zero.
// Errors wrap [strconv.ErrSyntax], [strconv.ErrRange] or [ErrNominalUnits].
func (f Format) Parse(s string) (time.Duration, error) {
	return f.parse([]byte(s), false)
}

// parse parses b in format f. With days set, [ISO8601] reads days and
// weeks as exactly 24 hours and 7 days.
func (f Format) parse(b []byte, days bool) (time.Duration, error) {
	switch f {
	case Units:
		return parseDurationUnits(b)
	case Sec, Milli, Micro, Nano:
		return parseDurationBase10(b, f.pow10())
	case ISO8601:
		return parseDurationISO8601(b, days)
	case Base60:
		return parseDurationBase60(b)
	case ProtoJSON:
		return parseDurationProto(b)
	}
	return 0, fmt.Errorf("jsonduration: invalid format %q", string(f))
}

// MarshalText implements [encoding.TextMarshaler], so that a format can be
// written to configuration. It returns an error if f is not valid.
func (f Format) MarshalText() ([]byte, error) {
	if !f.Valid() {
		return nil, fmt.Errorf("jsonduration: invalid format %q", string(f))
	}
	return []byte(f), nil
}

// UnmarshalText implements [encoding.TextUnmarshaler], so that a format
// in a JSON configuration is checked while encoding/json/v2 decodes it, and
// so that a format can come from a flag (see [flag.TextVar]). It accepts
// exactly the names of the formats, such as "iso8601", and rejects
// anything else, including other spellings such as "ISO8601", with an
// error that lists the names.
func (f *Format) UnmarshalText(b []byte) error {
	g := Format(b)
	if !g.Valid() {
		names := make([]string, 0, 8)
		for _, h := range Formats() {
			names = append(names, string(h))
		}
		return fmt.Errorf("jsonduration: unknown format %q, want one of %s", b, strings.Join(names, ", "))
	}
	*f = g
	return nil
}

// appendDurationBase10 appends d formatted as a decimal fractional number,
// where pow10 is a power-of-10 used to scale down the number.
func appendDurationBase10(b []byte, d time.Duration, pow10 uint64) []byte {
	b, n := mayAppendDurationSign(b, d)     // append sign
	whole, frac := bits.Div64(0, n, pow10)  // compute whole and frac fields
	b = strconv.AppendUint(b, whole, 10)    // append whole field
	return appendFracBase10(b, frac, pow10) // append frac field
}

// parseDurationBase10 parses d from a decimal fractional number,
// where pow10 is a power-of-10 used to scale up the number.
func parseDurationBase10(b []byte, pow10 uint64) (time.Duration, error) {
	suffix, neg := consumeSign(b, false)                     // consume sign
	wholeBytes, fracBytes := bytesCutByte(suffix, '.', true) // consume whole and frac fields
	whole, okWhole := parseUint(wholeBytes)                  // parse whole field; may overflow
	frac, okFrac := parseFracBase10(fracBytes, pow10)        // parse frac field
	hi, lo := bits.Mul64(whole, pow10)                       // overflow if hi > 0
	sum, co := bits.Add64(lo, frac, 0)                       // overflow if co > 0
	switch d := mayApplyDurationSign(sum, neg); {            // overflow if neg != (d < 0)
	case (!okWhole && whole != math.MaxUint64) || !okFrac:
		return 0, fmt.Errorf("invalid duration %q: %w", b, strconv.ErrSyntax)
	case !okWhole || hi > 0 || co > 0 || (neg != (d < 0) && d != 0):
		return 0, fmt.Errorf("invalid duration %q: %w", b, strconv.ErrRange)
	default:
		return d, nil
	}
}

// appendDurationISO8601 appends an ISO 8601 duration with a restricted grammar,
// where leading and trailing zeroes and zero-value designators are omitted.
// It only uses hour, minute, and second designators since ISO 8601 defines
// those as being "accurate", while year, month, week, and day are "nominal".
func appendDurationISO8601(b []byte, d time.Duration) []byte {
	if d == 0 {
		return append(b, "PT0S"...)
	}
	b, n := mayAppendDurationSign(b, d)
	b = append(b, "PT"...)
	n, nsec := bits.Div64(0, n, 1e9)   // compute nsec field
	n, sec := bits.Div64(0, n, 60)     // compute sec field
	hour, mins := bits.Div64(0, n, 60) // compute hour and min fields
	if hour > 0 {
		b = append(strconv.AppendUint(b, hour, 10), 'H')
	}
	if mins > 0 {
		b = append(strconv.AppendUint(b, mins, 10), 'M')
	}
	if sec > 0 || nsec > 0 {
		b = append(appendFracBase10(strconv.AppendUint(b, sec, 10), nsec, 1e9), 'S')
	}
	return b
}

// ErrNominalUnits reports an ISO 8601 duration that uses the year, month,
// week or day designators. Their length in time is not fixed, so they
// cannot be converted to a [time.Duration] exactly. [AcceptDays] allows
// days and weeks by fixing their length.
var ErrNominalUnits = errors.New("nominal year, month, week, or day units")

// parseDurationISO8601 parses a duration according to ISO 8601-1:2019,
// section 5.5.2.2 and 5.5.2.3, with the restrictions and extensions of
// encoding/json/v2 (see its source for the full list):
// a leading sign, '.' or ',' as decimal separator, leading zeros, lowercase
// designators, and a fraction only in the last component.
//
// It deviates from encoding/json/v2 in three ways:
//   - Any nominal unit (Y, M before T, W, D) is rejected outright, instead
//     of returning a best-effort value together with an error.
//   - Fractions of hours and minutes are computed exactly and truncated,
//     instead of being rounded through float64 (off by 1ns for about one
//     input in eight).
//   - A fraction must consist of digits only; encoding/json/v2 does not
//     check its first character and accepts "PT1.e5H" as one hour.
//
// With days set, days and weeks count as exactly 24 hours and 7 days, as
// in Java's Duration.parse; years and months are still rejected.
func parseDurationISO8601(b []byte, days bool) (time.Duration, error) {
	const day, week = 24 * time.Hour, 7 * 24 * time.Hour
	var invalid, overflow, nominal, sawFrac bool
	var sumNanos, n, co uint64

	// cutBytes is like [bytes.Cut], but uses either c0 or c1 as the separator.
	cutBytes := func(b []byte, c0, c1 byte) (prefix, suffix []byte, ok bool) {
		for i, c := range b {
			if c == c0 || c == c1 {
				return b[:i], b[i+1:], true
			}
		}
		return b, nil, false
	}

	// mayParseUnit attempts to parse another date or time number
	// identified by the desHi and desLo unit characters.
	// If the part is absent for current unit, it returns b as is.
	mayParseUnit := func(b []byte, desHi, desLo byte, unit time.Duration) []byte {
		number, suffix, ok := cutBytes(b, desHi, desLo)
		if !ok || sawFrac {
			return b // designator is not present or already saw fraction, which can only be in the last component
		}

		// Parse the number.
		// A fraction is allowed for the accurate units in the last part.
		whole, frac, ok := cutBytes(number, '.', ',')
		if ok {
			sawFrac = true
			n, ok = fracOfUnit(frac, uint64(unit)) // exact, never exceeds unit
			invalid = invalid || len(frac) == 0 || !ok || unit > time.Hour
			sumNanos, co = bits.Add64(sumNanos, n, 0) // overflow if co > 0
			overflow = overflow || co > 0
		}
		for len(whole) > 1 && whole[0] == '0' {
			whole = whole[len("0"):] // trim leading zeros
		}
		n, ok := parseUint(whole)                  // overflow if !ok && MaxUint64
		hi, lo := bits.Mul64(n, uint64(unit))      // overflow if hi > 0
		sumNanos, co = bits.Add64(sumNanos, lo, 0) // overflow if co > 0
		invalid = invalid || (!ok && n != math.MaxUint64)
		overflow = overflow || (!ok && n == math.MaxUint64) || hi > 0 || co > 0
		fixed := unit <= time.Hour || days && (unit == day || unit == week)
		nominal = nominal || !fixed
		return suffix
	}

	suffix, neg := consumeSign(b, true)
	prefix, suffix, okP := cutBytes(suffix, 'P', 'p')
	durDate, durTime, okT := cutBytes(suffix, 'T', 't')
	invalid = invalid || len(prefix) > 0 || !okP || (okT && len(durTime) == 0) || len(durDate)+len(durTime) == 0
	if len(durDate) > 0 { // nominal portion of the duration
		// Years and months are always rejected, so their lengths only need
		// to differ from those of days and weeks and exceed an hour.
		durDate = mayParseUnit(durDate, 'Y', 'y', 365*day)
		durDate = mayParseUnit(durDate, 'M', 'm', 30*day)
		durDate = mayParseUnit(durDate, 'W', 'w', week)
		durDate = mayParseUnit(durDate, 'D', 'd', day)
		invalid = invalid || len(durDate) > 0 // unknown elements
	}
	if len(durTime) > 0 { // accurate portion of the duration
		durTime = mayParseUnit(durTime, 'H', 'h', time.Hour)
		durTime = mayParseUnit(durTime, 'M', 'm', time.Minute)
		durTime = mayParseUnit(durTime, 'S', 's', time.Second)
		invalid = invalid || len(durTime) > 0 // unknown elements
	}
	d := mayApplyDurationSign(sumNanos, neg)
	overflow = overflow || (neg != (d < 0) && d != 0) // overflows signed duration

	switch {
	case invalid:
		return 0, fmt.Errorf("invalid ISO 8601 duration %q: %w", b, strconv.ErrSyntax)
	case nominal:
		return 0, fmt.Errorf("invalid ISO 8601 duration %q: %w", b, ErrNominalUnits)
	case overflow:
		return 0, fmt.Errorf("invalid ISO 8601 duration %q: %w", b, strconv.ErrRange)
	default:
		return d, nil
	}
}

// appendDurationBase60 appends d formatted with H:MM:SS.SSS notation.
func appendDurationBase60(b []byte, d time.Duration) []byte {
	b, n := mayAppendDurationSign(b, d)                      // append sign
	n, nsec := bits.Div64(0, n, 1e9)                         // compute nsec field
	n, sec := bits.Div64(0, n, 60)                           // compute sec field
	hour, mins := bits.Div64(0, n, 60)                       // compute hour and min fields
	b = strconv.AppendUint(b, hour, 10)                      // append hour field
	b = append(b, ':', '0'+byte(mins/10), '0'+byte(mins%10)) // append min field
	b = append(b, ':', '0'+byte(sec/10), '0'+byte(sec%10))   // append sec field
	return appendFracBase10(b, nsec, 1e9)                    // append nsec field
}

// parseDurationBase60 parses d formatted with H:MM:SS.SSS notation.
// The exact grammar is `-?(0|[1-9][0-9]*):[0-5][0-9]:[0-5][0-9]([.][0-9]+)?`.
func parseDurationBase60(b []byte) (time.Duration, error) {
	checkBase60 := func(b []byte) bool {
		return len(b) == 2 && ('0' <= b[0] && b[0] <= '5') && '0' <= b[1] && b[1] <= '9'
	}
	suffix, neg := consumeSign(b, false)                   // consume sign
	hourBytes, suffix := bytesCutByte(suffix, ':', false)  // consume hour field
	minBytes, suffix := bytesCutByte(suffix, ':', false)   // consume min field
	secBytes, nsecBytes := bytesCutByte(suffix, '.', true) // consume sec and nsec fields
	hour, okHour := parseUint(hourBytes)                   // parse hour field; may overflow
	mins := parseDec2(minBytes)                            // parse min field
	sec := parseDec2(secBytes)                             // parse sec field
	nsec, okNsec := parseFracBase10(nsecBytes, 1e9)        // parse nsec field
	n := uint64(mins)*60*1e9 + uint64(sec)*1e9 + nsec      // cannot overflow
	hi, lo := bits.Mul64(hour, 60*60*1e9)                  // overflow if hi > 0
	sum, co := bits.Add64(lo, n, 0)                        // overflow if co > 0
	switch d := mayApplyDurationSign(sum, neg); {          // overflow if neg != (d < 0)
	case (!okHour && hour != math.MaxUint64) || !checkBase60(minBytes) || !checkBase60(secBytes) || !okNsec:
		return 0, fmt.Errorf("invalid duration %q: %w", b, strconv.ErrSyntax)
	case !okHour || hi > 0 || co > 0 || (neg != (d < 0) && d != 0):
		return 0, fmt.Errorf("invalid duration %q: %w", b, strconv.ErrRange)
	default:
		return d, nil
	}
}

// appendDurationProto appends d in the protobuf JSON mapping of
// google.protobuf.Duration: seconds with 0, 3, 6 or 9 fractional digits
// followed by "s".
func appendDurationProto(b []byte, d time.Duration) []byte {
	b, n := mayAppendDurationSign(b, d)
	sec, nsec := bits.Div64(0, n, 1e9)
	b = strconv.AppendUint(b, sec, 10)
	switch {
	case nsec == 0:
	case nsec%1e6 == 0:
		b = appendPaddedBase10(append(b, '.'), nsec/1e6, 1e3)
	case nsec%1e3 == 0:
		b = appendPaddedBase10(append(b, '.'), nsec/1e3, 1e6)
	default:
		b = appendPaddedBase10(append(b, '.'), nsec, 1e9)
	}
	return append(b, 's')
}

// parseDurationProto parses the protobuf JSON mapping of
// google.protobuf.Duration. The grammar is `-?(0|[1-9][0-9]*)([.][0-9]{1,9})?s`.
func parseDurationProto(b []byte) (time.Duration, error) {
	number, ok := bytes.CutSuffix(b, []byte("s"))
	_, fracBytes := bytesCutByte(number, '.', true)
	if !ok || len(fracBytes) > len(".000000000") {
		return 0, fmt.Errorf("invalid protobuf duration %q: %w", b, strconv.ErrSyntax)
	}
	d, err := parseDurationBase10(number, 1e9)
	if err != nil {
		return 0, fmt.Errorf("invalid protobuf duration %q: %w", b, errors.Unwrap(err))
	}
	return d, nil
}

// unitLengths are the units of time.ParseDuration, in nanoseconds.
var unitLengths = map[string]uint64{
	"ns": uint64(time.Nanosecond),
	"us": uint64(time.Microsecond),
	"µs": uint64(time.Microsecond), // U+00B5, the micro sign
	"μs": uint64(time.Microsecond), // U+03BC, the Greek letter mu
	"ms": uint64(time.Millisecond),
	"s":  uint64(time.Second),
	"m":  uint64(time.Minute),
	"h":  uint64(time.Hour),
}

// parseDurationUnits parses the grammar of time.ParseDuration and accepts
// exactly the same inputs: `[-+]?([0-9]*(\.[0-9]*)?[a-z]+)+`, or "0" with
// an optional sign. Fractions are computed exactly and truncated, where
// time.ParseDuration goes through float64, and errors wrap
// strconv.ErrSyntax or strconv.ErrRange.
func parseDurationUnits(b []byte) (time.Duration, error) {
	syntaxError := func(detail string) error {
		return fmt.Errorf("invalid duration %q: %s%w", b, detail, strconv.ErrSyntax)
	}
	isDigit := func(c byte) bool { return '0' <= c && c <= '9' }

	s, neg := b, false
	if len(s) > 0 && (s[0] == '-' || s[0] == '+') {
		neg = s[0] == '-'
		s = s[1:]
	}
	if string(s) == "0" {
		return 0, nil
	}
	if len(s) == 0 {
		return 0, syntaxError("")
	}
	var sum, co2 uint64
	overflow := false
	for len(s) > 0 {
		if s[0] != '.' && !isDigit(s[0]) {
			return 0, syntaxError("")
		}
		i := 0
		for i < len(s) && isDigit(s[i]) {
			i++
		}
		whole, frac := s[:i], s[i:i]
		s = s[i:]
		if len(s) > 0 && s[0] == '.' {
			j := 1
			for j < len(s) && isDigit(s[j]) {
				j++
			}
			frac, s = s[1:j], s[j:]
		}
		if len(whole) == 0 && len(frac) == 0 {
			return 0, syntaxError("") // no digits, as in ".s"
		}
		i = 0
		for i < len(s) && s[i] != '.' && !isDigit(s[i]) {
			i++
		}
		if i == 0 {
			return 0, syntaxError("missing unit: ")
		}
		unit, ok := unitLengths[string(s[:i])]
		if !ok {
			return 0, syntaxError(fmt.Sprintf("unknown unit %q: ", s[:i]))
		}
		s = s[i:]

		n, ok := parseDigits(whole)
		part, _ := fracOfUnit(frac, unit) // digits only, below unit
		hi, lo := bits.Mul64(n, unit)
		lo, co1 := bits.Add64(lo, part, 0)
		sum, co2 = bits.Add64(sum, lo, 0)
		overflow = overflow || !ok || hi > 0 || co1 > 0 || co2 > 0 || sum > 1<<63
	}
	if overflow || !neg && sum > 1<<63-1 {
		return 0, fmt.Errorf("invalid duration %q: %w", b, strconv.ErrRange)
	}
	return mayApplyDurationSign(sum, neg), nil
}

// parseDigits parses b, a possibly empty run of decimal digits that may
// have leading zeros. It reports false if the value overflows a uint64.
func parseDigits(b []byte) (n uint64, ok bool) {
	for _, c := range b {
		if n > (math.MaxUint64-9)/10 {
			return 0, false
		}
		n = 10*n + uint64(c-'0')
	}
	return n, true
}

// mayAppendDurationSign appends a negative sign if n is negative.
func mayAppendDurationSign(b []byte, d time.Duration) ([]byte, uint64) {
	if d < 0 {
		b = append(b, '-')
		d *= -1
	}
	return b, uint64(d)
}

// mayApplyDurationSign inverts n if neg is specified.
func mayApplyDurationSign(n uint64, neg bool) time.Duration {
	if neg {
		return -1 * time.Duration(n)
	}
	return +1 * time.Duration(n)
}

// appendFracBase10 appends the fraction of n/max10,
// where max10 is a power-of-10 that is larger than n.
func appendFracBase10(b []byte, n, max10 uint64) []byte {
	if n == 0 {
		return b
	}
	return bytes.TrimRight(appendPaddedBase10(append(b, '.'), n, max10), "0")
}

// parseFracBase10 parses the fraction of n/max10,
// where max10 is a power-of-10 that is larger than n.
func parseFracBase10(b []byte, max10 uint64) (n uint64, ok bool) {
	switch {
	case len(b) == 0:
		return 0, true
	case len(b) < len(".0") || b[0] != '.':
		return 0, false
	}
	return parsePaddedBase10(b[len("."):], max10)
}

// appendPaddedBase10 appends a zero-padded encoding of n,
// where max10 is a power-of-10 that is larger than n.
func appendPaddedBase10(b []byte, n, max10 uint64) []byte {
	if n < max10/10 {
		// Formatting of n is shorter than log10(max10),
		// so add max10/10 to ensure the length is equal to log10(max10).
		i := len(b)
		b = strconv.AppendUint(b, n+max10/10, 10)
		b[i]-- // subtract the addition of max10/10
		return b
	}
	return strconv.AppendUint(b, n, 10)
}

// fracOfUnit returns ⌊0.frac × unit⌋, where frac holds the decimal digits
// after the separator. It is exact for any number of digits: the long
// multiplication runs from the last digit to the first, and the carry that
// leaves the first digit is the integer part. The carry stays below unit, so
// nothing overflows as long as 10×unit fits in a uint64.
// It reports false if frac contains a byte that is not a decimal digit.
func fracOfUnit(frac []byte, unit uint64) (n uint64, ok bool) {
	for i := len(frac) - 1; i >= 0; i-- {
		c := frac[i]
		if c < '0' || '9' < c {
			return 0, false
		}
		n = (uint64(c-'0')*unit + n) / 10
	}
	return n, true
}

// parsePaddedBase10 parses b as the zero-padded encoding of n,
// where max10 is a power-of-10 that is larger than n.
// Truncated suffix is treated as implicit zeros.
// Extended suffix is ignored, but verified to contain only digits.
func parsePaddedBase10(b []byte, max10 uint64) (n uint64, ok bool) {
	pow10 := uint64(1)
	for pow10 < max10 {
		n *= 10
		if len(b) > 0 {
			if b[0] < '0' || '9' < b[0] {
				return n, false
			}
			n += uint64(b[0] - '0')
			b = b[1:]
		}
		pow10 *= 10
	}
	if len(b) > 0 && len(bytes.TrimRight(b, "0123456789")) > 0 {
		return n, false // trailing characters are not digits
	}
	return n, true
}

// parseUint parses b as a decimal unsigned integer without sign or
// leading zeros. On overflow it reports (math.MaxUint64, false).
// Adapted from encoding/json/internal/jsonwire.ParseUint.
func parseUint(b []byte) (v uint64, ok bool) {
	const unsafeWidth = 20 // len(fmt.Sprint(uint64(math.MaxUint64)))
	var n int
	for ; len(b) > n && ('0' <= b[n] && b[n] <= '9'); n++ {
		v = 10*v + uint64(b[n]-'0')
	}
	switch {
	case n == 0 || len(b) != n || (b[0] == '0' && string(b) != "0"):
		return 0, false
	case n >= unsafeWidth && (b[0] != '1' || v < 1e19 || n > unsafeWidth):
		return math.MaxUint64, false
	}
	return v, true
}

// consumeSign consumes an optional leading negative or positive sign.
func consumeSign(b []byte, allowPlus bool) ([]byte, bool) {
	if len(b) > 0 {
		if b[0] == '-' {
			return b[len("-"):], true
		} else if b[0] == '+' && allowPlus {
			return b[len("+"):], false
		}
	}
	return b, false
}

// bytesCutByte is similar to bytes.Cut(b, []byte{c}),
// except c may optionally be included as part of the suffix.
func bytesCutByte(b []byte, c byte, include bool) ([]byte, []byte) {
	if i := bytes.IndexByte(b, c); i >= 0 {
		if include {
			return b[:i], b[i:]
		}
		return b[:i], b[i+1:]
	}
	return b, nil
}

// parseDec2 parses b as an unsigned, base-10, 2-digit number.
// The result is undefined if digits are not base-10.
func parseDec2(b []byte) byte {
	if len(b) < 2 {
		return 0
	}
	return 10*(b[0]-'0') + (b[1] - '0')
}
