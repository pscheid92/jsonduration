// Package jsonduration gives [time.Duration] a JSON representation in encoding/json/v2.
//
// encoding/json/v2 cannot encode a time.Duration on its own.
// It has no default representation (https://go.dev/issue/71631),
// and the `format` struct tag that used to select one was removed in Go 1.27 (https://go.dev/issue/79071).
// This package fills that gap, for encoding/json/v2 only, in two ways.
//
// # One option per call
//
// [With] returns options that apply one format to every time.Duration in a call:
// in struct fields, pointers, slices, arrays, map keys and values, and, when marshaling, in values held in an any.
// Your types stay as they are.
//
//	opts := jsonduration.With(jsonduration.ISO8601)
//	b, err := json.Marshal(v, opts) // {"ttl":"PT1H30M"}
//	err = json.Unmarshal(b, &v, opts)
//
// # One wrapper type per field
//
// Each format also has a wrapper type, such as [ISO8601Duration] or [SecDuration].
// It embeds a time.Duration, encodes itself without options, and writes and reads only its own format:
// [With], [AlsoAccept], and [AcceptDays] do not change it.
//
//	type Config struct {
//		Timeout jsonduration.ISO8601Duration `json:"timeout"`
//		Backoff jsonduration.SecDuration     `json:"backoff"`
//	}
//
// # Formats
//
// The six formats are those that encoding/json/v2 offered through `format`, with the same names and output:
// [Units], [Sec], [Milli], [Micro], [Nano], and [ISO8601]. [Nano] writes what encoding/json (v1) writes.
// A tag option such as `format:sec` is now an error: remove it, and pass With(Sec) or make the field a [SecDuration].
//
// In every numeric format, decoders that read JSON numbers as float64, such as JavaScript's JSON.parse,
// can lose the nanoseconds of long durations.
// So can encoding with jsontext.CanonicalizeRawInts or CanonicalizeRawFloats, which round numbers through float64.
//
// A [Format] can come from a JSON configuration or a flag,
// since it implements [encoding.TextUnmarshaler] and accepts only the exact names.
// Give it a default for a configuration that names none,
// and check a Format converted from a plain string with [Format.Valid].
//
// # Strictness
//
// Decoding accepts only the configured format.
// To also read other formats, for example while migrating, name them with [AlsoAccept].
//
// [ISO8601] rejects years, months, weeks, and days with [ErrCalendarUnits], since their length varies.
// Where a producer means 24 hours by a day, [AcceptDays] reads days and weeks.
//
// The numeric formats read JSON numbers without an exponent, such as 5400.5 but not 5.4005e3.
// Every format reads decimals exactly, without floating-point rounding, and drops digits below one nanosecond.
//
// # Errors
//
// An error means bad input, and a panic means code that uses this package wrongly.
//
// Every error about a duration that could not be read, from [Format.Parse] or from JSON decoding,
// matches [ErrInvalidDuration]. It names the input, the accepted formats, and the cause:
//
//	invalid duration "90s" in format iso8601: invalid syntax
//
// The cause is [strconv.ErrSyntax], [strconv.ErrRange], or [ErrCalendarUnits], which the error wraps.
// With formats from [AlsoAccept], it is a syntax error only if every format rejects the syntax.
// A JSON value of the wrong type, such as a number for ISO8601, has a cause of its own, which matches [ErrWrongType]
// and reads "want a JSON string".
// JSON decoding returns the error inside a json.SemanticError, as encoding/json/v2 does for other types.
// Its JSONPointer tells where in the input the duration is.
// A format name that [Format.UnmarshalText] does not know is an error that lists the valid names.
//
// An invalid [Format] makes [With], [Marshalers], [Unmarshalers], [Format.Format], and [Format.Parse] panic,
// also when it was passed to [AlsoAccept].
// So does accepting two numeric formats, as in With(Sec, AlsoAccept(Nano)).
// Only [Format.MarshalText] returns an error instead, as [encoding.TextMarshaler] requires.
//
// # Good to know
//
// Named types such as `type TTL time.Duration` are distinct types, so [With] does not affect them,
// and encoding/json/v2 writes them as plain integers of nanoseconds.
// To omit zero durations, use the `omitzero` tag option, since `omitempty` only omits values that encode as empty JSON.
// The `string` tag option quotes numeric formats, like json.StringifyNumbers, and has no effect on string formats.
// An explicit json.StringifyNumbers(false) turns it off for durations, unlike for other numbers,
// since json.GetOption then does not report the tag option.
package jsonduration
