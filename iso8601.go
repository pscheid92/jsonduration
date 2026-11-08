// Based on encoding/json/v2/arshal_time.go (Go 1.27.3), Copyright 2020 The Go Authors; see LICENSE.

package jsonduration

import (
	"strconv"
	"time"
)

// iso8601Codec writes and reads format [ISO8601], a JSON string such as "PT1H30M0.5S".
// With acceptDays, set by [AcceptDays] when decoding, a day counts as 24 hours and a week as 7 days.
type iso8601Codec struct {
	acceptDays bool
}

func (iso8601Codec) numeric() bool {
	return false
}

// appendTo writes d in hours, minutes, and seconds, leaving out those that are zero, such as "PT1H30M0.5S" or "-PT1M",
// and "PT0S" for zero.
func (iso8601Codec) appendTo(b []byte, d time.Duration) []byte {
	if d == 0 {
		return append(b, "PT0S"...)
	}

	b, n := appendSign(b, d)
	b = append(b, "PT"...)

	hours, n := n/uint64(time.Hour), n%uint64(time.Hour)
	minutes, n := n/uint64(time.Minute), n%uint64(time.Minute)
	seconds, nanos := n/uint64(time.Second), n%uint64(time.Second)

	if hours > 0 {
		b = append(strconv.AppendUint(b, hours, 10), 'H')
	}

	if minutes > 0 {
		b = append(strconv.AppendUint(b, minutes, 10), 'M')
	}

	if seconds > 0 || nanos > 0 {
		b = strconv.AppendUint(b, seconds, 10)
		b = append(appendFraction(b, nanos, uint64(time.Second)), 'S')
	}

	return b
}

// parse reads the restricted ISO 8601 profile of encoding/json/v2:
// an optional sign, "P", the date components, and then "T" and the clock components, in either case.
// Only the last component may have a fraction, after "."
// or ",", and of the accepted units only hours, minutes, and seconds can have one.
//
// With acceptDays, days and weeks count as 24 hours and 7 days.
//
// Unlike encoding/json/v2, it rejects calendar units outright,
// computes fractions of hours and minutes exactly instead of through float64,
// and rejects a fraction that is not all digits, as in "PT1.e5H".
func (iso iso8601Codec) parse(b []byte) (time.Duration, error) {
	s, neg := cutSign(b, true)

	if len(s) == 0 || s[0] != 'P' && s[0] != 'p' {
		return 0, strconv.ErrSyntax
	}

	date, clock, hasT := cutEither(s[1:], 'T', 't')
	if hasT && len(clock) == 0 || len(date) == 0 && len(clock) == 0 {
		return 0, strconv.ErrSyntax
	}

	sum := isoSum{acceptDays: iso.acceptDays}
	var ok bool
	for _, c := range isoDate {
		if date, ok = sum.take(date, c); !ok {
			return 0, strconv.ErrSyntax
		}
	}

	for _, c := range isoClock {
		if clock, ok = sum.take(clock, c); !ok {
			return 0, strconv.ErrSyntax
		}
	}

	if len(date) > 0 || len(clock) > 0 {
		return 0, strconv.ErrSyntax
	}

	d, fits := signed(sum.total, neg)
	switch {
	case sum.calendarUnits:
		return 0, ErrCalendarUnits
	case !fits:
		return 0, strconv.ErrRange
	}

	return d, nil
}

// An isoComponent is a number followed by its designator, in either case, such as "30M" for 30 minutes.
type isoComponent struct {
	upper, lower byte
	kind         isoKind
	unit         uint64
}

// An isoKind says whether a component has a fixed length.
type isoKind int

const (
	// clockUnit is an hour, a minute, or a second, which always have a fixed length and may take a fraction.
	clockUnit isoKind = iota

	// dayUnit is a day or a week. With acceptDays it has a fixed length, but takes no fraction.
	// Without acceptDays it is a calendar unit.
	dayUnit

	// calendarUnit is a year or a month, which have no fixed length. Its unit is unused.
	calendarUnit
)

const (
	day  = 24 * time.Hour
	week = 7 * day
)

var (
	isoDate = []isoComponent{
		{'Y', 'y', calendarUnit, 0},
		{'M', 'm', calendarUnit, 0},
		{'W', 'w', dayUnit, uint64(week)},
		{'D', 'd', dayUnit, uint64(day)},
	}

	isoClock = []isoComponent{
		{'H', 'h', clockUnit, uint64(time.Hour)},
		{'M', 'm', clockUnit, uint64(time.Minute)},
		{'S', 's', clockUnit, uint64(time.Second)},
	}
)

// isoSum adds up the components of an ISO 8601 duration.
// Calendar units and a total out of range are reported only once the whole duration is known to be well formed,
// since a syntax error comes first.
type isoSum struct {
	acceptDays    bool
	total         uint64
	calendarUnits bool
	hadFraction   bool
}

// take reads the component c, such as "30M", from the start of s, adds it to the sum and returns the rest of s.
// parse calls it once per component, in order. If s does not contain the designator of c, take returns s unchanged.
// It reports false for a malformed component.
func (sum *isoSum) take(s []byte, c isoComponent) ([]byte, bool) {
	number, rest, found := cutEither(s, c.upper, c.lower)
	if !found {
		return s, true
	}

	// Only the last component may have a fraction.
	if sum.hadFraction {
		return nil, false
	}

	// This also rejects components out of order: in "1M1H", the text before the H is "1M1", which is not a number.
	whole, fraction, hasFraction := cutEither(number, '.', ',')
	if !isDigits(whole) || hasFraction && !isDigits(fraction) {
		return nil, false
	}

	sum.hadFraction = hasFraction

	isCalendar := c.kind == calendarUnit || c.kind == dayUnit && !sum.acceptDays
	switch {
	case isCalendar:
		sum.calendarUnits = true
	case c.kind == dayUnit && hasFraction:
		return nil, false
	default:
		sum.total = add(sum.total, scale(whole, fraction, c.unit))
	}

	return rest, true
}
