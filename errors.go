package jsonduration

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalidDuration is matched by every error about a duration that could not be read,
// from [Format.Parse] and from JSON decoding.
// Use it with [errors.Is] to tell bad input from other errors, for example to answer a request with 400 Bad Request.
// Such an error reads like this:
//
//	invalid duration "90s" in format iso8601: invalid syntax
var ErrInvalidDuration = errors.New("invalid duration")

// ErrCalendarUnits is wrapped by the error for an ISO 8601 duration in years, months, weeks, or days,
// such as "P1D" or "P0.5D".
// Their length varies, a month from 28 to 31 days and a day from 23 to 25 hours, so they have no exact [time.Duration].
// Where a day means 24 hours, [AcceptDays] reads days and weeks.
var ErrCalendarUnits = errors.New("calendar units have no fixed length")

// errInvalidDuration builds every error about a duration that could not be read:
// text failed in formats, such as "iso8601" or "iso8601 or nano", because of cause.
func errInvalidDuration(text []byte, formats string, cause error) error {
	return fmt.Errorf("%w %s in format %s: %w", ErrInvalidDuration, quote(text), formats, cause)
}

// quote quotes at most maxQuoted bytes of input, since an error message may go back to whoever sent the input.
func quote(input []byte) string {
	if len(input) > maxQuoted {
		return strconv.Quote(string(input[:maxQuoted])) + "..."
	}

	return strconv.Quote(string(input))
}

// maxQuoted is how many bytes of a bad input an error quotes, far more than any valid duration or format name has.
const maxQuoted = 64

// errMissingUnit is the cause for a number without a unit in format Units, such as "1".
var errMissingUnit = fmt.Errorf("missing unit: %w", strconv.ErrSyntax)

// errUnknownUnit is the cause for a unit that format Units does not know, such as "x" in "1x".
func errUnknownUnit(name []byte) error {
	return fmt.Errorf("unknown unit %s: %w", quote(name), strconv.ErrSyntax)
}

// errWrongKind is the cause for a JSON value of a kind that no accepted format reads, such as a number for ISO8601.
func errWrongKind(want string) error {
	return fmt.Errorf("want a JSON %s", want)
}

// errUnknownFormat reports a format name, from a flag or a configuration, that is not the name of a format.
func errUnknownFormat(name []byte) error {
	validFormats := joinNames(Formats(), ", ")
	return fmt.Errorf("unknown format %s, want one of %s", quote(name), validFormats)
}

// errInvalidFormat reports a Format that is not one of the formats, such as one converted from a string.
func errInvalidFormat(f Format) error {
	return fmt.Errorf("invalid format %q", string(f))
}

// errAmbiguousNumeric reports two numeric formats for one decoder, which could not tell their JSON numbers apart.
func errAmbiguousNumeric(first, second Format) error {
	return fmt.Errorf("cannot accept both %s and %s: a JSON number would be ambiguous", first, second)
}

// joinNames lists the names of formats, separated by sep, for error messages.
func joinNames(formats []Format, sep string) string {
	names := make([]string, len(formats))
	for i, f := range formats {
		names[i] = string(f)
	}

	return strings.Join(names, sep)
}
