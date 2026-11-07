package jsonduration

import (
	"strconv"
	"time"
)

// unitsCodec writes and reads format [Units], a JSON string such as "1h30m0.5s".
type unitsCodec struct{}

func (unitsCodec) numeric() bool {
	return false
}

// appendTo writes d as [time.Duration.String] does, such as "1h30m0.5s".
func (unitsCodec) appendTo(b []byte, d time.Duration) []byte {
	return append(b, d.String()...)
}

// parse reads exactly what time.ParseDuration reads:
// an optional sign and numbers with units, such as "1h30m" or "-1.5us", or "0".
// Unlike time.ParseDuration, it computes fractions exactly, not through float64.
func (unitsCodec) parse(b []byte) (time.Duration, error) {
	s, neg := cutSign(b, true)
	if string(s) == "0" {
		return 0, nil
	}

	if len(s) == 0 {
		return 0, strconv.ErrSyntax
	}

	var total uint64
	for len(s) > 0 {
		var whole, fraction, name []byte
		whole, s = cutWhile(s, isDigit)
		if len(s) > 0 && s[0] == '.' {
			fraction, s = cutWhile(s[1:], isDigit)
		}

		if len(whole) == 0 && len(fraction) == 0 {
			return 0, strconv.ErrSyntax
		}

		name, s = cutWhile(s, isUnitByte)
		if len(name) == 0 {
			return 0, errMissingUnit
		}

		unit, known := unitLengths[string(name)]
		if !known {
			return 0, errUnknownUnit(name)
		}

		total = add(total, scale(whole, fraction, unit))
	}

	d, fits := signed(total, neg)
	if !fits {
		return 0, strconv.ErrRange
	}

	return d, nil
}

// unitLengths maps the units of time.ParseDuration to nanoseconds.
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

// isUnitByte reports whether c can be part of a unit name in format Units, which runs up to the next digit or point.
func isUnitByte(c byte) bool {
	return c != '.' && !isDigit(c)
}
