package jsonduration

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"reflect"
	"time"
)

// shared holds what [With], [Marshalers] and [Unmarshalers] return for one set of arguments.
// Its values never change and are safe for concurrent use.
type shared struct {
	marshalers   *json.Marshalers
	unmarshalers *json.Unmarshalers
	options      json.Options
}

// sharedByFormat holds the values of each format without decoding options, which is how most calls ask for them.
// They are built once, so that such a call allocates nothing and every caller gets the same ones:
// encoding/json/v2 remembers per value which types it has seen.
var sharedByFormat = func() map[Format]*shared {
	byFormat := make(map[Format]*shared, len(Formats()))
	for _, f := range Formats() {
		byFormat[f] = newShared(f, decodeConfig{}, nil)
	}

	return byFormat
}()

// sharedFor returns the values for the arguments of [With]: the ones of the format if no option changes anything,
// and new ones otherwise. It panics as [With] does.
func sharedFor(f Format, opts []DecodeOption) *shared {
	plain, ok := sharedByFormat[f]
	if !ok {
		panic(errInvalidFormat(f))
	}

	config := configFrom(opts)
	if len(config.accepted) == 0 && !config.acceptDays {
		return plain
	}

	// Options change only how durations are read, so the marshalers are those of the format alone.
	return newShared(f, config, plain.marshalers)
}

// newShared builds the values for format f and the options of a call. Without marshalers, it builds those too.
// It panics if a second numeric format is accepted.
func newShared(f Format, config decodeConfig, marshalers *json.Marshalers) *shared {
	dc := newDecoder(f, config)
	dc.goType = reflect.TypeFor[time.Duration]()

	if marshalers == nil {
		c := f.mustCodec()
		marshalers = json.MarshalToFunc(func(enc *jsontext.Encoder, d time.Duration) error {
			return encode(enc, c, d)
		})
	}

	unmarshalers := json.UnmarshalFromFunc(dc.decode)

	return &shared{
		marshalers:   marshalers,
		unmarshalers: unmarshalers,
		options:      json.JoinOptions(json.WithMarshalers(marshalers), json.WithUnmarshalers(unmarshalers)),
	}
}
