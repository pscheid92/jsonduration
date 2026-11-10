// Based on encoding/json/v2/arshal_time.go (Go 1.27.3), Copyright 2020 The Go Authors; see NOTICE.

package jsonduration

import (
	"bytes"
	"strconv"
	"time"
)

// The numeric formats [Sec], [Milli], [Micro], and [Nano] hold a JSON number of units, such as 5400.5 seconds.
// Their functions take the length of one unit in nanoseconds.

// appendDecimal writes d as a decimal number of units.
func appendDecimal(b []byte, d time.Duration, unit uint64) []byte {
	b, n := appendSign(b, d)
	b = strconv.AppendUint(b, n/unit, 10)
	return appendFraction(b, n%unit, unit)
}

// parseDecimal reads a number of units as JSON writes numbers, but without an exponent:
// an optional minus sign, 0 or digits without leading zeros, and optionally a point and more digits.
func parseDecimal(b []byte, unit uint64) (time.Duration, error) {
	s, neg := cutSign(b, false)

	whole, fraction, hasPoint := bytes.Cut(s, []byte("."))
	if !isNatural(whole) || hasPoint && !isDigits(fraction) {
		return 0, strconv.ErrSyntax
	}

	d, fits := signed(scale(whole, fraction, unit), neg)
	if !fits {
		return 0, strconv.ErrRange
	}

	return d, nil
}
