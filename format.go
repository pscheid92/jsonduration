package jsonduration

import "time"

// Format is a way to write a [time.Duration] in JSON, such as [ISO8601] for "PT1H30M".
// Use it with [With], or convert single durations with [Format.Format] and [Format.Parse].
//
// A Format is its name, so that a flag ([flag.TextVar]) or a JSON configuration can hold one, and both reject unknown
// names. As with time.Month(13), a value that names no format can therefore exist, and using it is a programming error:
// [With] panics on an invalid Format, such as the empty one that decoding leaves for a missing name or null,
// so give it a default, as in cmp.Or(cfg.DurationFormat, jsonduration.ISO8601).
// A Format converted from a string, as in Format(os.Getenv("DURATION_FORMAT")), is not checked,
// so check it with [Format.Valid] first.
type Format string

const (
	// Units writes a JSON string as [time.Duration.String] does, such as "1h30m0.5s".
	// It reads everything [time.ParseDuration] reads, such as "90s" or "1.5h".
	Units Format = "units"

	// Sec writes a JSON number of seconds, such as 5400.5, with decimals down to nanoseconds.
	// It does not read exponents such as 1e-05, which some encoders write for small numbers.
	Sec Format = "sec"

	// Milli writes a JSON number of milliseconds, such as 5400500, or 1.5 for 1500µs.
	Milli Format = "milli"

	// Micro writes a JSON number of microseconds, such as 5400500000, or 1.5 for 1500ns.
	Micro Format = "micro"

	// Nano writes a JSON number of nanoseconds, such as 5400500000000, as encoding/json (v1) does.
	// Beyond about 104 days, JavaScript and other decoders that read numbers as float64 can no longer hold it exactly.
	Nano Format = "nano"

	// ISO8601 writes a JSON string in ISO 8601, such as "PT1H30M0.5S", in hours, minutes, and seconds.
	// It also reads forms such as "PT90M" and "PT1.5H", designators in either case, and a decimal comma, as in
	// "pt1,5s". It rejects years, months, weeks, and days, whose length varies, unless their count is zero;
	// see [AcceptDays].
	// Only one sign in front is allowed, as in "-PT30S" or "+PT30S", so Java's negative durations such as "PT-30S"
	// are rejected.
	ISO8601 Format = "iso8601"
)

// Formats returns all formats, for example to list them in a flag's usage.
func Formats() []Format {
	return []Format{Units, Sec, Milli, Micro, Nano, ISO8601}
}

// Valid reports whether f is one of the formats. Names are lowercase, so Format("ISO8601") is not valid.
func (f Format) Valid() bool {
	_, ok := f.codec()
	return ok
}

// Format returns the duration as text in format f, without JSON quotes,
// such as "PT1M30S" for 90 seconds in ISO8601 or "90" in Sec.
// Use it to write a duration outside JSON the same way, such as in a log line. It panics if f is not valid.
func (f Format) Format(d time.Duration) string {
	var buf [maxPossibleOutputLength]byte
	return string(f.mustCodec().appendTo(buf[:0], d))
}

// Parse reads a duration in format f, without JSON quotes, such as "5400.5" in Sec or "PT1H30M" in ISO8601.
// It is the inverse of [Format.Format] and reads only format f,
// since [AlsoAccept] and [AcceptDays] apply to JSON decoding only. Digits below one nanosecond are truncated.
// An error matches [ErrInvalidDuration]. It panics if f is not valid.
func (f Format) Parse(s string) (time.Duration, error) {
	text := []byte(s)

	d, cause := f.mustCodec().parse(text)
	if cause != nil {
		return 0, errInvalidDuration(text, string(f), cause)
	}

	return d, nil
}

// MarshalText returns the name of f, such as "iso8601", so that a configuration holds a Format by its name.
// It returns an error if f is not valid.
func (f Format) MarshalText() ([]byte, error) {
	if !f.Valid() {
		return nil, errInvalidFormat(f)
	}

	return []byte(f), nil
}

// UnmarshalText sets f from a name such as "iso8601",
// so that a flag ([flag.TextVar]) or a JSON configuration can name a format.
// Any other name is an error that lists the valid ones and leaves f unchanged.
func (f *Format) UnmarshalText(b []byte) error {
	newFormat := Format(b)

	if !newFormat.Valid() {
		return errUnknownFormat(b)
	}

	*f = newFormat
	return nil
}

// A codec writes and reads durations in one format, without JSON quotes. Format.codec maps a format to its codec.
// Each kind of codec has its functions in its own file.
//
// It is a struct and not an interface, so that the compiler sees which function a call reaches:
// through an interface, every buffer handed to a codec would move to the heap.
type codec struct {
	kind codecKind

	// unit is the length in nanoseconds of the unit that a decimal codec counts in.
	unit uint64

	// acceptDays makes an ISO 8601 codec read a day as 24 hours and a week as 7 days, for decoding with AcceptDays.
	acceptDays bool
}

// A codecKind tells the codecs apart that share no code.
type codecKind uint8

const (
	unitsCodec codecKind = iota
	decimalCodec
	iso8601Codec
)

// numeric reports whether the format writes a JSON number rather than a string.
func (c codec) numeric() bool {
	return c.kind == decimalCodec
}

// appendTo appends d to b.
func (c codec) appendTo(b []byte, d time.Duration) []byte {
	switch c.kind {
	case decimalCodec:
		return appendDecimal(b, d, c.unit)
	case iso8601Codec:
		return appendISO8601(b, d)
	default:
		return appendUnits(b, d)
	}
}

// parse reads b.
// If b is not a duration in this format, it returns the cause:
// [strconv.ErrSyntax], [strconv.ErrRange], [ErrCalendarUnits], or an error that wraps one of them.
// errInvalidDuration turns the cause into the error a caller sees.
func (c codec) parse(b []byte) (time.Duration, error) {
	switch c.kind {
	case decimalCodec:
		return parseDecimal(b, c.unit)
	case iso8601Codec:
		return parseISO8601(b, c.acceptDays)
	default:
		return parseUnits(b)
	}
}

// maxPossibleOutputLength is the length of the longest text any codec writes. TestLongestOutput checks it.
const maxPossibleOutputLength = 27

// codec returns the codec of format f, and false if f is not valid.
func (f Format) codec() (codec, bool) {
	switch f {
	case Units:
		return codecUnits, true
	case Sec:
		return codecSec, true
	case Milli:
		return codecMilli, true
	case Micro:
		return codecMicro, true
	case Nano:
		return codecNano, true
	case ISO8601:
		return codecISO8601, true
	}

	return codec{}, false
}

// mustCodec returns the codec of format f, and panics if f is not valid.
func (f Format) mustCodec() codec {
	c, ok := f.codec()
	if !ok {
		panic(errInvalidFormat(f))
	}

	return c
}

// The codecs of the formats.
var (
	codecUnits   = codec{kind: unitsCodec}
	codecSec     = codec{kind: decimalCodec, unit: uint64(time.Second)}
	codecMilli   = codec{kind: decimalCodec, unit: uint64(time.Millisecond)}
	codecMicro   = codec{kind: decimalCodec, unit: uint64(time.Microsecond)}
	codecNano    = codec{kind: decimalCodec, unit: uint64(time.Nanosecond)}
	codecISO8601 = codec{kind: iso8601Codec}

	// codecISO8601Days also reads days and weeks, for decoding with AcceptDays.
	codecISO8601Days = codec{kind: iso8601Codec, acceptDays: true}
)
