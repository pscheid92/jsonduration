// Based on encoding/json/v2/arshal_time.go (Go 1.27.3), Copyright 2020 The Go Authors; see LICENSE.

package jsonduration

import (
	"bytes"
	"strconv"
	"time"
)

// decimalCodec writes and reads the numeric formats [Sec], [Milli], [Micro], and [Nano]:
// a JSON number of units, such as 5400.5 seconds. unit is the length of one unit in nanoseconds.
type decimalCodec struct {
	unit uint64
}

func (decimalCodec) numeric() bool {
	return true
}

// appendTo writes d as a decimal number of units.
func (c decimalCodec) appendTo(b []byte, d time.Duration) []byte {
	b, n := appendSign(b, d)
	b = strconv.AppendUint(b, n/c.unit, 10)
	return appendFraction(b, n%c.unit, c.unit)
}

// parse reads a number of units as JSON writes numbers, but without an exponent:
// an optional minus sign, 0 or digits without leading zeros, and optionally a point and more digits.
func (c decimalCodec) parse(b []byte) (time.Duration, error) {
	s, neg := cutSign(b, false)

	whole, fraction, hasPoint := bytes.Cut(s, []byte("."))
	if !isNatural(whole) || hasPoint && !isDigits(fraction) {
		return 0, strconv.ErrSyntax
	}

	d, fits := signed(scale(whole, fraction, c.unit), neg)
	if !fits {
		return 0, strconv.ErrRange
	}

	return d, nil
}
