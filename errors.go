package jsonduration

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
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
//
// A count of zero is not an error, since it is no time at all whatever the length of the unit: "P0D" and
// "P0Y0M0DT1H" are read in every mode. Some producers write zero as "P0D", and some write every component.
var ErrCalendarUnits = errors.New("calendar units have no fixed length")

// ErrWrongType is matched by the error for a JSON value of a type that no accepted format reads, such as a number for
// [ISO8601] or a string for [Sec]. The error says which type it wants, as in "want a JSON string".
// Use it to tell a client that sent the wrong type from one that sent a malformed duration.
var ErrWrongType = errors.New("wrong JSON type")

// errInvalidDuration builds every error about a duration that could not be read:
// text failed in formats, such as "iso8601" or "iso8601 or nano", because of cause.
func errInvalidDuration(text []byte, formats string, cause error) error {
	return &durationError{input: excerptOf(text), formats: formats, cause: cause}
}

// A durationError is the error of [errInvalidDuration]. It matches [ErrInvalidDuration] and wraps its cause.
// It builds its message when asked and not before, since many callers only want to know that the input was bad,
// and building the message costs more than everything else about rejecting a duration.
type durationError struct {
	input   excerpt
	formats string
	cause   error
}

func (e *durationError) Error() string {
	return ErrInvalidDuration.Error() + " " + e.input.quote() + " in format " + e.formats + ": " + e.cause.Error()
}

func (e *durationError) Is(target error) bool {
	return target == ErrInvalidDuration
}

func (e *durationError) Unwrap() error {
	return e.cause
}

// An excerpt is as much of a bad input as an error quotes: at most maxQuoted bytes,
// since an error message may go back to whoever sent the input.
type excerpt struct {
	text string
	// cut reports that the input was longer than text.
	cut bool
}

// excerptOf copies the excerpt out of input, which may be memory that the caller uses again.
// It does not cut a UTF-8 character in two, which would show as a replacement character that the input never held.
func excerptOf(input []byte) excerpt {
	if len(input) <= maxQuoted {
		return excerpt{text: string(input)}
	}

	// A character has at most utf8.UTFMax bytes, so the one around the cut starts at most utf8.UTFMax-1 bytes
	// before it. Text without a first byte there is not UTF-8, and is cut where it is.
	cut := maxQuoted
	for back := range utf8.UTFMax {
		if utf8.RuneStart(input[maxQuoted-back]) {
			cut = maxQuoted - back
			break
		}
	}

	return excerpt{text: string(input[:cut]), cut: true}
}

// quote returns the excerpt in quotes, followed by "..." if the input was longer.
func (e excerpt) quote() string {
	if e.cut {
		return strconv.Quote(e.text) + "..."
	}

	return strconv.Quote(e.text)
}

// quote quotes at most maxQuoted bytes of input.
func quote(input []byte) string {
	return excerptOf(input).quote()
}

// maxQuoted is how many bytes of a bad input an error quotes, far more than any valid duration or format name has.
const maxQuoted = 64

// errMissingUnit is the cause for a number without a unit in format Units, such as "1".
var errMissingUnit = fmt.Errorf("missing unit: %w", strconv.ErrSyntax)

// errUnknownUnit is the cause for a unit that format Units does not know, such as "x" in "1x".
func errUnknownUnit(name []byte) error {
	return &unknownUnitError{name: excerptOf(name)}
}

// An unknownUnitError is the error of [errUnknownUnit]. It wraps [strconv.ErrSyntax], and like a durationError
// it builds its message when asked.
type unknownUnitError struct {
	name excerpt
}

func (e *unknownUnitError) Error() string {
	return "unknown unit " + e.name.quote() + ": " + strconv.ErrSyntax.Error()
}

func (e *unknownUnitError) Unwrap() error {
	return strconv.ErrSyntax
}

// A wrongKindError is the cause for a JSON value of a kind that no accepted format reads, such as a number for
// ISO8601. It names the kinds that are read, such as "string or number", and matches [ErrWrongType].
type wrongKindError struct {
	want string
}

func (e wrongKindError) Error() string {
	return "want a JSON " + e.want
}

func (e wrongKindError) Is(target error) bool {
	return target == ErrWrongType
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
