package jsonduration

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"reflect"
	"sync/atomic"
	"time"
)

// shared holds what [With], [Marshalers] and [Unmarshalers] return for one set of arguments.
// Its values never change and are safe for concurrent use, so every call with these arguments gets the same ones.
// That saves more than building them: encoding/json/v2 remembers per value which types it has seen.
type shared struct {
	marshalers   *json.Marshalers
	unmarshalers *json.Unmarshalers
	options      json.Options
}

// sharedByArgs holds the shared values of every set of arguments: by the number of the written format less one,
// by the set of the other accepted formats, and by whether days are accepted.
// A set of arguments that is not valid, such as one with two numeric formats, keeps its nil pointer.
// An array needs no hashing and no lock, and at 768 pointers it is small.
var sharedByArgs [numFormats][1 << numFormats][2]atomic.Pointer[shared]

// sharedFor returns the shared values for the arguments of [With], and builds them on the first call.
// It panics as [With] does.
func sharedFor(f Format, opts []DecodeOption) *shared {
	config := configFrom(opts)

	number := f.number()
	if number == 0 {
		panic(errInvalidFormat(f))
	}

	// Accepting the written format again changes nothing, so it gets no entry of its own.
	config.accepted &^= f.bit()

	days := 0
	if config.acceptDays {
		days = 1
	}

	slot := &sharedByArgs[number-1][config.accepted][days]
	if s := slot.Load(); s != nil {
		return s
	}

	// This panics for arguments that are not valid, before anything is stored.
	dc := newDecoder(f, config)
	dc.goType = reflect.TypeFor[time.Duration]()

	s := &shared{unmarshalers: json.UnmarshalFromFunc(dc.decode)}
	if config == (decodeConfig{}) {
		c := f.mustCodec()
		s.marshalers = json.MarshalToFunc(func(enc *jsontext.Encoder, d time.Duration) error {
			return encode(enc, c, d)
		})
	} else {
		// Options change only how durations are read, so the marshalers are those of the format alone.
		s.marshalers = sharedFor(f, nil).marshalers
	}

	s.options = json.JoinOptions(json.WithMarshalers(s.marshalers), json.WithUnmarshalers(s.unmarshalers))

	// If another goroutine was faster, its values are the ones to share.
	slot.CompareAndSwap(nil, s)
	return slot.Load()
}
