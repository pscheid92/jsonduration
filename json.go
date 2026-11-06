// Copyright 2026 Patrick Scheid. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonduration

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"slices"
	"strings"
	"time"
)

// With returns options for encoding/json/v2 that marshal and unmarshal
// every [time.Duration] using format f: struct fields, pointers, slice and
// array elements, map keys and values, and values held in an any when
// marshaling. Decoding accepts only f, unless [AlsoAccept] adds formats.
// With panics under the same conditions as [Unmarshalers].
//
// The options are safe for concurrent use, so they can be created once,
// for example in a package-level variable, and shared:
//
//	var jsonOpts = jsonduration.With(jsonduration.ISO8601)
//
//	b, err := json.Marshal(v, jsonOpts)
//	err = json.Unmarshal(b, &v, jsonOpts)
//
// If you also pass your own json.WithMarshalers or json.WithUnmarshalers,
// combine them with [Marshalers] and [Unmarshalers] through
// json.JoinMarshalers and json.JoinUnmarshalers instead, because a later
// WithMarshalers option replaces an earlier one.
func With(f Format, opts ...DecodeOption) json.Options {
	return json.JoinOptions(
		json.WithMarshalers(Marshalers(f)),
		json.WithUnmarshalers(Unmarshalers(f, opts...)),
	)
}

// Marshalers returns the marshaler for [time.Duration] in format f.
// It panics if f is not valid.
func Marshalers(f Format) *json.Marshalers {
	mustValid(f)
	return json.MarshalToFunc(func(enc *jsontext.Encoder, d time.Duration) error {
		return encode(enc, f, d)
	})
}

// encode writes d in format f, as a JSON string where JSON or the options
// require one, and as a JSON number otherwise.
func encode(enc *jsontext.Encoder, f Format, d time.Duration) error {
	quote := !f.IsNumeric() || stringify(enc.Options(), isObjectName(enc.StackIndex(enc.StackDepth())))
	// The longest output, "-PT2562047H47M16.854775808S", is 29 bytes.
	// No format produces a byte that needs escaping in a JSON string,
	// so the quotes are added directly.
	var buf [32]byte
	b := buf[:0]
	if quote {
		b = append(b, '"')
	}
	b = f.Append(b, d)
	if quote {
		b = append(b, '"')
	}
	return enc.WriteValue(b)
}

// A DecodeOption changes how [With] and [Unmarshalers] decode.
// Encoding always uses the single format passed to them.
type DecodeOption interface {
	apply(*decodeConfig)
}

type decodeConfig struct {
	accept []Format
	days   bool
}

// alsoAccept is the DecodeOption that AlsoAccept returns.
type alsoAccept []Format

func (a alsoAccept) apply(c *decodeConfig) { c.accept = append(c.accept, a...) }

// acceptDays is the DecodeOption that AcceptDays returns.
type acceptDays struct{}

func (acceptDays) apply(c *decodeConfig) { c.days = true }

// AcceptDays makes [ISO8601] decoding through [With] and [Unmarshalers]
// read days as exactly 24 hours and weeks as exactly 7 days, as Java's
// Duration.parse does for days, so that "P1D" is 24h and "P1DT12H" is 36h.
// Years and months are still rejected with [ErrNominalUnits], and days
// and weeks take no fraction. Encoding never writes days. AcceptDays has
// no effect on the other formats, and [Format.Parse] and
// [ISO8601Duration] stay strict.
//
// Only use it where the producer means 24 hours by a day: a calendar day
// can be 23 or 25 hours long across a daylight saving change.
func AcceptDays() DecodeOption {
	return acceptDays{}
}

// AlsoAccept makes decoding accept the given formats in addition to the
// one that is written. Use it to read data written in an older format while
// writing a new one:
//
//	// Stored data holds v1 nanoseconds; new data is written as ISO 8601.
//	opts := jsonduration.With(jsonduration.ISO8601, jsonduration.AlsoAccept(jsonduration.Nano))
//
// String formats never conflict: no text is valid in two of them with
// different values. Where two accept the same text, such as "1.5s" in
// [Units] and [ProtoJSON], they agree on its value. Numeric formats do
// conflict, since a JSON number such as 90 could be seconds or
// nanoseconds, so at most one numeric format may be involved. [With] and
// [Unmarshalers] panic if there are two, or if a format is not valid.
func AlsoAccept(formats ...Format) DecodeOption {
	return alsoAccept(formats)
}

// Unmarshalers returns the unmarshaler for [time.Duration] in format f and
// in any format added with [AlsoAccept]. It panics if a format is not
// valid or if two numeric formats are involved.
//
// Decoding is strict: only the accepted formats are tried, and a numeric
// format accepts a JSON string only where it would also write one (object
// names, or with json.StringifyNumbers). JSON null sets the duration to
// zero, as encoding/json/v2 does for other types.
func Unmarshalers(f Format, opts ...DecodeOption) *json.Unmarshalers {
	return json.UnmarshalFromFunc(newDecoder(f, opts).decode)
}

// decoder decodes a duration written in any of its formats.
type decoder struct {
	formats []Format // the written format first, then the accepted ones
	numeric bool     // whether one of the formats is numeric
	days    bool     // whether ISO 8601 days and weeks are accepted
	names   string   // the formats for messages, e.g. "iso8601 or nano"
}

func newDecoder(f Format, opts []DecodeOption) *decoder {
	var c decodeConfig
	for _, opt := range opts {
		if opt != nil {
			opt.apply(&c)
		}
	}
	u := new(decoder)
	var num Format
	var names []string
	for _, g := range append([]Format{f}, c.accept...) {
		mustValid(g)
		if slices.Contains(u.formats, g) {
			continue
		}
		if g.IsNumeric() {
			if num != "" {
				panic(fmt.Sprintf("jsonduration: cannot accept both %s and %s: a JSON number would be ambiguous", num, g))
			}
			num = g
		}
		u.formats = append(u.formats, g)
		names = append(names, string(g))
	}
	u.numeric = num != ""
	u.days = c.days
	u.names = strings.Join(names, " or ")
	return u
}

func (u *decoder) decode(dec *jsontext.Decoder, d *time.Duration) error {
	// Numeric formats are JSON strings in object names and with
	// json.StringifyNumbers, and JSON numbers everywhere else.
	quoteNumbers := u.numeric && stringify(dec.Options(), isObjectName(dec.StackIndex(dec.StackDepth())))
	val, err := dec.ReadValue()
	if err != nil {
		return err
	}
	k := val.Kind()
	var text []byte
	switch k {
	case 'n':
		*d = 0
		return nil
	case '"':
		// Without escapes the text is the value between its quotes.
		// val is valid until the next read, which parse does not do.
		if text = val[1 : len(val)-1]; bytes.IndexByte(text, '\\') >= 0 {
			if text, err = jsontext.AppendUnquote(nil, val); err != nil {
				return err
			}
		}
	case '0':
		text = val
	}
	var firstErr error
	for _, f := range u.formats {
		if !accepts(f, k, quoteNumbers) {
			continue
		}
		v, err := f.parse(text, u.days)
		if err == nil {
			*d = v
			return nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	switch {
	case firstErr == nil:
		return u.kindError(k, quoteNumbers)
	case len(u.formats) > 1:
		return fmt.Errorf("%w (accepted: %s)", firstErr, u.names)
	default:
		return firstErr
	}
}

// accepts reports whether format f decodes a JSON value of kind k.
func accepts(f Format, k jsontext.Kind, quoteNumbers bool) bool {
	if !f.IsNumeric() || quoteNumbers {
		return k == '"'
	}
	return k == '0'
}

// kindError reports a JSON value of a kind that no format accepts.
func (u *decoder) kindError(k jsontext.Kind, quoteNumbers bool) error {
	var str, num bool
	for _, f := range u.formats {
		str = str || accepts(f, '"', quoteNumbers)
		num = num || accepts(f, '0', quoteNumbers)
	}
	want := "string"
	switch {
	case str && num:
		want = "string or number"
	case num:
		want = "number"
	}
	return fmt.Errorf("%s duration must be a JSON %s, got %v", u.names, want, k)
}

// isObjectName reports whether the next token is an object member name,
// given the innermost container kind and the number of tokens in it.
func isObjectName(k jsontext.Kind, n int64) bool {
	return k == '{' && n%2 == 0
}

func stringify(opts json.Options, objectName bool) bool {
	if objectName {
		return true
	}
	v, _ := json.GetOption(opts, json.StringifyNumbers)
	return v
}

func mustValid(f Format) {
	if !f.Valid() {
		panic(fmt.Sprintf("jsonduration: invalid format %q", string(f)))
	}
}
