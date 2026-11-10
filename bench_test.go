package jsonduration

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"io"
	"testing"
	"time"
)

// formatted keeps results of Format alive, as a caller would, so that they cannot stay on the stack.
var formatted string

func TestAllocations(t *testing.T) {
	for _, f := range Formats() {
		t.Run(string(f), func(t *testing.T) {
			opts := With(f)
			text := f.Format(d90)
			enc := jsontext.NewEncoder(io.Discard, opts)

			in, err := json.Marshal(d90, opts)
			if err != nil {
				t.Fatal(err)
			}

			var d time.Duration
			tests := []struct {
				name string
				want float64
				fn   func() error
			}{
				// Format allocates the string it returns.
				{"Format", 1, func() error { formatted = f.Format(d90); return nil }},
				{"Parse", 0, func() error { _, err := f.Parse(text); return err }},
				{"encode", 0, func() error { return encode(enc, f.mustCodec(), d90) }},
				{"Unmarshal", 0, func() error { return json.Unmarshal(in, &d, opts) }},
			}

			for _, tt := range tests {
				var err error
				allocs := testing.AllocsPerRun(100, func() { err = tt.fn() })
				if err != nil {
					t.Fatalf("%s: %v", tt.name, err)
				}

				if allocs != tt.want {
					t.Errorf("%s allocs = %v, want %v", tt.name, allocs, tt.want)
				}
			}
		})
	}
}

// The benchmarks below are the ones BENCHMARKS.md reports. Their names are <operation>/<format>/<way>,
// with "baseline" in place of the format for what to compare with.

// A field is a struct with one field, the smallest value that shows a duration in its usual place.
type field[T any] struct {
	D T
}

// textDuration is what one writes by hand without this package: a type with the methods of encoding.TextMarshaler
// and encoding.TextUnmarshaler, in the format of time.Duration.String.
type textDuration time.Duration

func (d textDuration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

func (d *textDuration) UnmarshalText(b []byte) error {
	parsed, err := time.ParseDuration(string(b))
	*d = textDuration(parsed)

	return err
}

// nanoFuncs passes functions as With does, and does as little in them as a function can: it writes and reads
// nanoseconds. What it costs more than an int64 is what the standard library takes to call a function for a type.
var nanoFuncs = json.JoinOptions(
	json.WithMarshalers(json.MarshalToFunc(func(enc *jsontext.Encoder, d time.Duration) error {
		return enc.WriteToken(jsontext.Int(int64(d)))
	})),
	json.WithUnmarshalers(json.UnmarshalFromFunc(func(dec *jsontext.Decoder, d *time.Duration) error {
		tok, err := dec.ReadToken()
		if err != nil {
			return err
		}

		n, err := tok.Int()
		*d = time.Duration(n)

		return err
	})),
)

// A config is a value as a service might read at its start: two durations and a list of them among other fields.
type config[T any] struct {
	Name     string
	Address  string
	Retries  int
	Verbose  bool
	Timeout  T
	Interval T
	Backoff  []T
}

// configOf returns the config of the benchmarks, with its durations as values of T.
func configOf[T any](wrap func(time.Duration) T) config[T] {
	return config[T]{
		Name:     "billing",
		Address:  "billing.internal:8443",
		Retries:  3,
		Verbose:  true,
		Timeout:  wrap(30 * time.Second),
		Interval: wrap(d90),
		Backoff:  []T{wrap(time.Second / 10), wrap(time.Second), wrap(10 * time.Second)},
	}
}

// benchMarshal measures json.Marshal of a pointer to v, so that handing v over does not allocate.
func benchMarshal[T any](b *testing.B, v T, opts ...json.Options) {
	b.Helper()
	b.ReportAllocs()

	for b.Loop() {
		if _, err := json.Marshal(&v, opts...); err != nil {
			b.Fatal(err)
		}
	}
}

// benchUnmarshal measures json.Unmarshal of in into a T, which is reused between calls.
func benchUnmarshal[T any](b *testing.B, in []byte, opts ...json.Options) {
	b.Helper()
	b.ReportAllocs()

	var got T
	for b.Loop() {
		if err := json.Unmarshal(in, &got, opts...); err != nil {
			b.Fatal(err)
		}
	}
}

// mustMarshal returns the JSON of v, as input for a benchmark of decoding.
func mustMarshal(b *testing.B, v any, opts ...json.Options) []byte {
	b.Helper()

	out, err := json.Marshal(v, opts...)
	if err != nil {
		b.Fatal(err)
	}

	return out
}

// benchValue measures decoding in into a T, or encoding v if in is nil.
func benchValue[T any](b *testing.B, v T, in []byte, opts ...json.Options) {
	b.Helper()

	if in == nil {
		benchMarshal(b, v, opts...)
		return
	}

	benchUnmarshal[T](b, in, opts...)
}

// benchWrapper measures a value whose durations have the wrapper type of format f: a field, or a whole config.
// It decodes in, or encodes the value if in is nil.
func benchWrapper(b *testing.B, f Format, whole bool, in []byte) {
	b.Helper()

	switch f {
	case Units:
		benchWrapped(b, func(d time.Duration) UnitsDuration { return UnitsDuration{d} }, whole, in)
	case Sec:
		benchWrapped(b, func(d time.Duration) SecDuration { return SecDuration{d} }, whole, in)
	case Milli:
		benchWrapped(b, func(d time.Duration) MilliDuration { return MilliDuration{d} }, whole, in)
	case Micro:
		benchWrapped(b, func(d time.Duration) MicroDuration { return MicroDuration{d} }, whole, in)
	case Nano:
		benchWrapped(b, func(d time.Duration) NanoDuration { return NanoDuration{d} }, whole, in)
	case ISO8601:
		benchWrapped(b, func(d time.Duration) ISO8601Duration { return ISO8601Duration{d} }, whole, in)
	}
}

func benchWrapped[T any](b *testing.B, wrap func(time.Duration) T, whole bool, in []byte) {
	b.Helper()

	if whole {
		benchValue(b, configOf(wrap), in)
		return
	}

	benchValue(b, field[T]{wrap(d90)}, in)
}

func BenchmarkMarshal(b *testing.B) {
	v := field[time.Duration]{d90}

	b.Run("baseline/int64", func(b *testing.B) { benchMarshal(b, field[int64]{int64(d90)}) })
	b.Run("baseline/nano_funcs", func(b *testing.B) { benchMarshal(b, v, nanoFuncs) })
	b.Run("baseline/text_methods", func(b *testing.B) { benchMarshal(b, field[textDuration]{textDuration(d90)}) })

	for _, f := range Formats() {
		opts := With(f)

		b.Run(string(f)+"/options", func(b *testing.B) { benchMarshal(b, v, opts) })
		b.Run(string(f)+"/wrapper", func(b *testing.B) { benchWrapper(b, f, false, nil) })
	}
}

func BenchmarkUnmarshal(b *testing.B) {
	v := field[time.Duration]{d90}
	nanos := mustMarshal(b, field[int64]{int64(d90)})

	b.Run("baseline/int64", func(b *testing.B) { benchUnmarshal[field[int64]](b, nanos) })
	b.Run("baseline/nano_funcs", func(b *testing.B) { benchUnmarshal[field[time.Duration]](b, nanos, nanoFuncs) })
	b.Run("baseline/text_methods", func(b *testing.B) {
		benchUnmarshal[field[textDuration]](b, mustMarshal(b, v, With(Units)))
	})

	for _, f := range Formats() {
		opts := With(f)
		in := mustMarshal(b, v, opts)

		b.Run(string(f)+"/options", func(b *testing.B) { benchUnmarshal[field[time.Duration]](b, in, opts) })
		b.Run(string(f)+"/wrapper", func(b *testing.B) { benchWrapper(b, f, false, in) })
	}
}

// BenchmarkUnmarshalAlsoAccept measures what a second accepted format costs.
func BenchmarkUnmarshalAlsoAccept(b *testing.B) {
	v := field[time.Duration]{d90}
	iso, units := mustMarshal(b, v, With(ISO8601)), mustMarshal(b, v, With(Units))
	nanos := mustMarshal(b, field[int64]{int64(d90)})

	isoOnly := With(ISO8601)
	isoOrUnits := With(ISO8601, AlsoAccept(Units))
	isoOrNano := With(ISO8601, AlsoAccept(Nano))

	// The written format is tried first, so the other one costs nothing here.
	b.Run("one_format", func(b *testing.B) { benchUnmarshal[field[time.Duration]](b, iso, isoOnly) })
	b.Run("first_of_two", func(b *testing.B) { benchUnmarshal[field[time.Duration]](b, iso, isoOrUnits) })

	// ISO8601 has to reject the string before Units reads it.
	b.Run("second_of_two_strings", func(b *testing.B) { benchUnmarshal[field[time.Duration]](b, units, isoOrUnits) })

	// A number is never tried as ISO8601, which reads strings only.
	b.Run("second_is_a_number", func(b *testing.B) { benchUnmarshal[field[time.Duration]](b, nanos, isoOrNano) })
}

// BenchmarkConfig measures a value with other fields than durations. With options, the standard library asks for
// every field whether a function applies to it, which a wrapper type avoids.
func BenchmarkConfig(b *testing.B) {
	durations := configOf(func(d time.Duration) time.Duration { return d })
	ints := configOf(func(d time.Duration) int64 { return int64(d) })
	nanos := mustMarshal(b, ints)

	b.Run("Marshal/baseline/int64", func(b *testing.B) { benchMarshal(b, ints) })
	b.Run("Marshal/baseline/nano_funcs", func(b *testing.B) { benchMarshal(b, durations, nanoFuncs) })
	b.Run("Unmarshal/baseline/int64", func(b *testing.B) { benchUnmarshal[config[int64]](b, nanos) })
	b.Run("Unmarshal/baseline/nano_funcs", func(b *testing.B) {
		benchUnmarshal[config[time.Duration]](b, nanos, nanoFuncs)
	})

	for _, f := range Formats() {
		opts := With(f)
		in := mustMarshal(b, durations, opts)

		b.Run("Marshal/"+string(f)+"/options", func(b *testing.B) { benchMarshal(b, durations, opts) })
		b.Run("Marshal/"+string(f)+"/wrapper", func(b *testing.B) { benchWrapper(b, f, true, nil) })
		b.Run("Unmarshal/"+string(f)+"/options", func(b *testing.B) {
			benchUnmarshal[config[time.Duration]](b, in, opts)
		})
		b.Run("Unmarshal/"+string(f)+"/wrapper", func(b *testing.B) { benchWrapper(b, f, true, in) })
	}
}

func BenchmarkFormat(b *testing.B) {
	for _, f := range Formats() {
		b.Run(string(f), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				formatted = f.Format(d90)
			}
		})
	}
}

func BenchmarkParse(b *testing.B) {
	for _, f := range Formats() {
		s := f.Format(d90)

		b.Run(string(f), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := f.Parse(s); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
