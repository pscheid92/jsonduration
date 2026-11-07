// Based on encoding/json/v2/arshal_time.go (Go 1.27.3), Copyright 2020 The Go Authors; see LICENSE.

package jsonduration

import (
	"math"
	"math/bits"
	"slices"
	"time"
)

// maxAbs is the largest absolute value of a Duration, that of math.MinInt64.
// It does not fit in an int64, so absolute values are uint64.
const maxAbs uint64 = 1 << 63

// appendSign appends a minus sign if d is negative, and returns the absolute value of d.
func appendSign(b []byte, d time.Duration) (withSign []byte, abs uint64) {
	if d == math.MinInt64 {
		return append(b, '-'), maxAbs
	}

	if d < 0 {
		return append(b, '-'), uint64(-d)
	}

	return b, uint64(d)
}

// appendFraction appends n/unit as decimals without trailing zeros, such as ".005" for 5/1000, and nothing for 0.
// unit is a power of ten greater than n.
func appendFraction(b []byte, n, unit uint64) []byte {
	if n == 0 {
		return b
	}

	b = append(b, '.')
	for place := unit / 10; n > 0; place /= 10 {
		digit := n / place
		b = append(b, decimalDigits[digit])
		n -= digit * place
	}

	return b
}

const decimalDigits = "0123456789"

// cutSign removes a leading minus sign, or a plus sign if allowPlus is set, and reports whether it was a minus.
func cutSign(s []byte, allowPlus bool) ([]byte, bool) {
	if len(s) == 0 {
		return s, false
	}

	if s[0] == '-' {
		return s[1:], true
	}

	if s[0] == '+' && allowPlus {
		return s[1:], false
	}

	return s, false
}

// cutEither is like [bytes.Cut], but cuts around the first c1 or c2.
func cutEither(s []byte, c1, c2 byte) (before, after []byte, found bool) {
	for i, c := range s {
		if c == c1 || c == c2 {
			return s[:i], s[i+1:], true
		}
	}

	return s, nil, false
}

// cutWhile splits s after its leading bytes for which keep is true.
func cutWhile(s []byte, keep func(byte) bool) (head, rest []byte) {
	i := 0
	for i < len(s) && keep(s[i]) {
		i++
	}

	return s[:i], s[i:]
}

func isDigit(c byte) bool {
	return '0' <= c && c <= '9'
}

// isDigits reports whether s is one or more decimal digits.
func isDigits(s []byte) bool {
	digits, rest := cutWhile(s, isDigit)
	return len(digits) > 0 && len(rest) == 0
}

// isNatural reports whether s is a number without leading zeros, such as 0 or 42.
func isNatural(s []byte) bool {
	return isDigits(s) && (s[0] != '0' || len(s) == 1)
}

// outOfRange stands for any value above maxAbs.
// mul and add return it instead of overflowing, so the parsers check the range only once, in signed.
const outOfRange = maxAbs + 1

// scale returns whole.fraction × unit in whole nanoseconds. whole and fraction are digits and may be empty.
func scale(whole, fraction []byte, unit uint64) uint64 {
	n := parseDigits(whole)
	wholePart := mul(n, unit)
	fractionPart := fracOfUnit(fraction, unit)

	return add(wholePart, fractionPart)
}

// parseDigits parses decimal digits, which may be empty or have leading zeros.
// It stops at outOfRange: past maxAbs/10, one more digit would pass maxAbs.
func parseDigits(digits []byte) uint64 {
	var n uint64
	for _, c := range digits {
		if n > maxAbs/10 {
			return outOfRange
		}

		n = 10*n + uint64(c-'0')
	}

	return n
}

// fracOfUnit returns 0.frac × unit, rounded down, exactly for any number of digits.
// It goes from the last digit to the first and divides by ten after each, as for 0.25 hours:
//
//	5:  5 × 3600s / 10          = 1800s
//	2: (2 × 3600s + 1800s) / 10 =  900s
//
// 10 × unit must fit in a uint64.
func fracOfUnit(frac []byte, unit uint64) uint64 {
	var n uint64

	for _, digit := range slices.Backward(frac) {
		n = (uint64(digit-'0')*unit + n) / 10
	}

	return n
}

// mul returns a × b, or outOfRange if that is greater than maxAbs.
func mul(a, b uint64) uint64 {
	high, low := bits.Mul64(a, b)
	if high > 0 || low > maxAbs {
		return outOfRange
	}

	return low
}

// add returns a + b, or outOfRange if that is greater than maxAbs.
func add(a, b uint64) uint64 {
	if a > maxAbs || b > maxAbs-a {
		return outOfRange
	}

	return a + b
}

// signed returns the Duration with absolute value abs and the given sign, and false if that is out of range.
func signed(abs uint64, neg bool) (time.Duration, bool) {
	if neg && abs == maxAbs {
		return math.MinInt64, true
	}

	if abs > math.MaxInt64 {
		return 0, false
	}

	d := time.Duration(abs)
	if neg {
		d = -d
	}

	return d, true
}
