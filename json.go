package jsonduration

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"time"
)

// With returns options that make encoding/json/v2 write and read every [time.Duration] in format f:
// in struct fields, pointers, slices, arrays, map keys and values, and, when marshaling, in values held in an any.
// It panics if f is not valid or two numeric formats are accepted.
//
// Decoding is strict.
// It accepts only f, unless [AlsoAccept] adds formats,
// and reads a numeric format from a JSON string only where it is written as one:
// in object names and with json.StringifyNumbers. JSON null sets the duration to zero.
//
// The options are safe for concurrent use.
// With builds them on the first call and returns the same ones for the same arguments afterwards,
// so a call costs a lookup and no allocation, and it makes no difference whether you keep the options or ask again:
//
//	var jsonOpts = jsonduration.With(jsonduration.ISO8601)
//
//	b, err := json.Marshal(v, jsonOpts)
//	err = json.Unmarshal(b, &v, jsonOpts)
//
// To combine them with your own marshalers or unmarshalers, use [Marshalers] and [Unmarshalers].
func With(f Format, opts ...DecodeOption) json.Options {
	return sharedFor(f, opts).options
}

// Marshalers returns the writing half of [With], to join with your own marshalers through json.JoinMarshalers:
// a second json.WithMarshalers option would replace the first. It panics if f is not valid.
func Marshalers(f Format) *json.Marshalers {
	return sharedFor(f, nil).marshalers
}

// Unmarshalers returns the reading half of [With], to join with your own unmarshalers through json.JoinUnmarshalers:
// a second json.WithUnmarshalers option would replace the first. It panics as [With] does.
func Unmarshalers(f Format, opts ...DecodeOption) *json.Unmarshalers {
	return sharedFor(f, opts).unmarshalers
}

// A DecodeOption changes how [With] and [Unmarshalers] read durations:
// [AlsoAccept] adds formats, and [AcceptDays] reads ISO 8601 days. Writing always uses the one format given.
type DecodeOption interface {
	// apply returns the configuration with the option applied.
	// It takes a value, not a pointer, so that the configuration of a call can stay on the stack.
	apply(decodeConfig) decodeConfig
}

// AlsoAccept makes decoding accept the given formats besides the one that is written,
// for example to read old data while writing a new format:
//
//	// Stored data holds v1 nanoseconds; new data is written as ISO 8601.
//	opts := jsonduration.With(jsonduration.ISO8601, jsonduration.AlsoAccept(jsonduration.Nano))
//
// String formats never conflict, but numeric ones do:
// a JSON number such as 90 could be seconds or nanoseconds, so at most one numeric format may be accepted.
//
// Accept only the formats you need, since data that two systems read differently is a common source of security bugs.
//
// The order of the formats makes no difference. AlsoAccept panics if a format is not valid.
func AlsoAccept(formats ...Format) DecodeOption {
	var accepted formatSet
	for _, f := range formats {
		bit := f.bit()
		if bit == 0 {
			panic(errInvalidFormat(f))
		}

		accepted |= bit
	}

	return alsoAccept(accepted)
}

// AcceptDays makes [ISO8601] decoding read a day as exactly 24 hours and a week as 7 days: "P1DT12H" is 36h.
// Python's Pydantic and .NET's XmlConvert write every duration of a day or more with days.
// Years and months are still rejected unless their count is zero, and days and weeks take no fraction.
// Only JSON decoding is affected: encoding never writes days, and [Format.Parse] and [ISO8601Duration] stay strict.
//
// Use it only where the producer means 24 hours by a day:
// across a daylight saving change, a calendar day has 23 or 25 hours.
func AcceptDays() DecodeOption {
	return acceptDays{}
}

// decodeConfig is what the options of a call add up to.
type decodeConfig struct {
	accepted   formatSet
	acceptDays bool
}

// configFrom applies the options in order, skipping nil ones.
func configFrom(opts []DecodeOption) decodeConfig {
	var config decodeConfig
	for _, opt := range opts {
		if opt != nil {
			config = opt.apply(config)
		}
	}

	return config
}

// alsoAccept is the option of [AlsoAccept]. It is one byte, which Go puts into an interface without allocating.
type alsoAccept formatSet

func (a alsoAccept) apply(c decodeConfig) decodeConfig {
	c.accepted |= formatSet(a)
	return c
}

type acceptDays struct{}

func (acceptDays) apply(c decodeConfig) decodeConfig {
	c.acceptDays = true
	return c
}

// encode writes d with codec c:
// as a JSON number for a numeric format, unless the position or the options call for a JSON string.
// It writes into the encoder's own buffer, which the encoder hands out for this.
func encode(enc *jsontext.Encoder, c codec, d time.Duration) error {
	quoted := !c.numeric() || numbersQuoted(enc)
	b := enc.AvailableBuffer()
	if quoted {
		b = append(b, '"')
	}

	b = c.appendTo(b, d)
	if quoted {
		b = append(b, '"')
	}

	return enc.WriteValue(b)
}

// position is what jsontext.Encoder and jsontext.Decoder tell about the value they write or read next.
type position interface {
	Options() json.Options
	StackDepth() int
	StackIndex(depth int) (jsontext.Kind, int64)
}

// numbersQuoted reports whether a number at the current position is written as a JSON string:
// in an object name, as JSON requires, or with json.StringifyNumbers.
// Inside an object, names and values alternate, so after an even number of tokens a name comes next.
func numbersQuoted(p position) bool {
	kind, tokens := p.StackIndex(p.StackDepth())
	if kind == '{' && tokens%2 == 0 {
		return true
	}

	stringify, _ := json.GetOption(p.Options(), json.StringifyNumbers)
	return stringify
}
