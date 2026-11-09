package jsonduration

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"time"
)

// decoder reads a duration in any of its formats, the written one first.
// codecs holds the codec of each format, set up for the decoding options.
type decoder struct {
	formats    []Format
	codecs     []codec
	hasNumeric bool

	// goType is the Go type that errors name. If it is nil, encoding/json/v2 names the type it decodes into:
	// the wrapper type for a wrapper type, but *time.Duration for Unmarshalers.
	goType reflect.Type
}

// newDecoder applies the options and checks the formats.
// It leaves out duplicates and panics on an invalid format or a second numeric one.
func newDecoder(f Format, opts []DecodeOption) *decoder {
	config := configFrom(opts)
	formats := append([]Format{f}, config.alsoAccept...)

	dc := &decoder{}
	var numeric Format
	for _, g := range formats {
		c := g.mustCodec()
		if slices.Contains(dc.formats, g) {
			continue
		}

		if c.numeric() {
			if numeric != "" {
				panic(errAmbiguousNumeric(numeric, g))
			}

			numeric = g
		}

		if g == ISO8601 && config.acceptDays {
			c = codecISO8601Days
		}

		dc.formats = append(dc.formats, g)
		dc.codecs = append(dc.codecs, c)
	}

	dc.hasNumeric = numeric != ""
	return dc
}

// decode is the unmarshaler: it reads one JSON value and hands it to parse.
// It reports errors as encoding/json/v2 does for other types: in a json.SemanticError that names the JSON kind.
func (dc *decoder) decode(dec *jsontext.Decoder, d *time.Duration) error {
	// Before ReadValue: afterwards, the position is the one after the value.
	quoted := dc.hasNumeric && numbersQuoted(dec)

	val, err := dec.ReadValue()
	if err != nil {
		return err
	}

	kind := val.Kind()
	if kind == 'n' {
		*d = 0
		return nil
	}

	parsed, err := dc.parse(val, quoted)
	if err != nil {
		return &json.SemanticError{JSONKind: kind, GoType: dc.goType, Err: err}
	}

	*d = parsed
	return nil
}

// parse tries the accepted formats that read the kind of val, in order, and returns the first duration one reads.
// If all fail, it reports the first cause, or that none reads this kind.
// A cause other than a syntax error goes first, since that format recognized the duration but not its value.
func (dc *decoder) parse(val jsontext.Value, quoted bool) (time.Duration, error) {
	text, err := unquote(val)
	if err != nil {
		return 0, errInvalidDuration(text, dc.names(), err)
	}

	var reported error
	for _, c := range dc.codecs {
		if wantedKind(c, quoted) != val.Kind() {
			continue
		}

		d, cause := c.parse(text)
		if cause == nil {
			return d, nil
		}

		if reported == nil || errors.Is(reported, strconv.ErrSyntax) && !errors.Is(cause, strconv.ErrSyntax) {
			reported = cause
		}
	}

	if reported == nil {
		reported = dc.wrongKind(quoted)
	}

	return 0, errInvalidDuration(text, dc.names(), reported)
}

// unquote returns the text of a JSON string without its quotes, or val itself for other kinds.
// Without escapes in the string, the result shares memory with val.
func unquote(val jsontext.Value) ([]byte, error) {
	if val.Kind() != '"' {
		return val, nil
	}

	if text := val[1 : len(val)-1]; bytes.IndexByte(text, '\\') < 0 {
		return text, nil
	}

	// The decoder has checked the string, so an error here is invalid UTF-8 that its options allow.
	// AppendUnquote then returns the text with U+FFFD in place of it, and no format reads that.
	text, err := jsontext.AppendUnquote(nil, val)
	if err != nil {
		return text, fmt.Errorf("%w: %w", err, strconv.ErrSyntax)
	}

	return text, nil
}

// wantedKind returns the JSON kind that codec c reads:
// a number for a numeric format, unless numbers are quoted at this position, and a string otherwise.
func wantedKind(c codec, quoted bool) jsontext.Kind {
	if c.numeric() && !quoted {
		return '0'
	}

	return '"'
}

// wrongKind is the cause for a JSON value of a kind that no accepted format reads, such as a number for ISO8601.
func (dc *decoder) wrongKind(quoted bool) error {
	var readsString, readsNumber bool
	for _, c := range dc.codecs {
		kind := wantedKind(c, quoted)
		readsString = readsString || kind == '"'
		readsNumber = readsNumber || kind == '0'
	}

	want := "string"
	switch {
	case readsString && readsNumber:
		want = "string or number"
	case readsNumber:
		want = "number"
	}

	return wrongKindError{want}
}

// names returns the formats for error messages, such as "iso8601 or nano".
func (dc *decoder) names() string {
	return joinNames(dc.formats, " or ")
}
