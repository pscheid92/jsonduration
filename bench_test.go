package jsonduration

import (
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
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

// An error allocates the input it quotes and itself. Its message is built when asked for, which this does not do.
func TestRejectedDurationAllocations(t *testing.T) {
	var err error

	parse := func(f Format, in string) func() {
		return func() { _, err = f.Parse(in) }
	}

	// An unknown unit is an error of its own inside the other, with the name of the unit.
	tests := []struct {
		name string
		want float64
		fn   func()
	}{
		{"ISO8601.Parse", 2, parse(ISO8601, "1h30m")},
		{"Units.Parse with an unknown unit", 4, parse(Units, "5fortnights")},
	}

	for _, tt := range tests {
		if allocs := testing.AllocsPerRun(100, tt.fn); allocs != tt.want || err == nil {
			t.Errorf("%s: %v allocs with error %v, want %v allocs and an error", tt.name, allocs, err, tt.want)
		}
	}

	// What encoding/json/v2 allocates around the error is its own business, and differs between platforms.
	// So this measures the decoder alone, which builds the names of its formats once and not for every error.
	for _, opts := range [][]DecodeOption{nil, {AlsoAccept(Nano, Units), AcceptDays()}} {
		dc := newDecoder(ISO8601, configFrom(opts))
		in := jsontext.Value(`"PT1X"`)

		allocs := testing.AllocsPerRun(100, func() { _, err = dc.parse(in, false) })
		if allocs != 2 || !errors.Is(err, ErrInvalidDuration) {
			t.Errorf("decoder in format %s: %v allocs with error %v, want 2 allocs and an error", dc.names, allocs, err)
		}
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

// nanoOption is the option of the standard library that writes a time.Duration as nanoseconds.
var nanoOption = jsonv1.FormatDurationAsNano(true)

// nanoFuncs does the same with functions, as With does, and does as little in them as a function can. What it costs
// more than nanoOption is what the standard library takes to call a function for a type.
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

// benchWrapperConfig measures a config whose durations have the wrapper type of format f: decoding in, or encoding
// such a config if in is nil.
func benchWrapperConfig(b *testing.B, f Format, in []byte) {
	b.Helper()

	switch f {
	case Units:
		benchConfigOf(b, func(d time.Duration) UnitsDuration { return UnitsDuration{d} }, in)
	case Sec:
		benchConfigOf(b, func(d time.Duration) SecDuration { return SecDuration{d} }, in)
	case Milli:
		benchConfigOf(b, func(d time.Duration) MilliDuration { return MilliDuration{d} }, in)
	case Micro:
		benchConfigOf(b, func(d time.Duration) MicroDuration { return MicroDuration{d} }, in)
	case Nano:
		benchConfigOf(b, func(d time.Duration) NanoDuration { return NanoDuration{d} }, in)
	case ISO8601:
		benchConfigOf(b, func(d time.Duration) ISO8601Duration { return ISO8601Duration{d} }, in)
	}
}

func benchConfigOf[T any](b *testing.B, wrap func(time.Duration) T, in []byte) {
	b.Helper()

	if in == nil {
		benchMarshal(b, configOf(wrap))
		return
	}

	benchUnmarshal[config[T]](b, in)
}

// manyDurations returns n durations of different lengths, about as long as d90.
func manyDurations(n int) []time.Duration {
	durations := make([]time.Duration, n)
	for i := range durations {
		durations[i] = d90 + time.Duration(i)*time.Second
	}

	return durations
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

// benchMarshalWrite measures json.MarshalWrite, which returns no JSON and so does not allocate for it.
func benchMarshalWrite[T any](b *testing.B, v T, opts ...json.Options) {
	b.Helper()
	b.ReportAllocs()

	for b.Loop() {
		if err := json.MarshalWrite(io.Discard, &v, opts...); err != nil {
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

// benchWrapper measures a field of the wrapper type of format f: decoding in, or encoding such a field if in is nil.
func benchWrapper(b *testing.B, f Format, in []byte) {
	b.Helper()

	switch f {
	case Units:
		benchField(b, UnitsDuration{d90}, in)
	case Sec:
		benchField(b, SecDuration{d90}, in)
	case Milli:
		benchField(b, MilliDuration{d90}, in)
	case Micro:
		benchField(b, MicroDuration{d90}, in)
	case Nano:
		benchField(b, NanoDuration{d90}, in)
	case ISO8601:
		benchField(b, ISO8601Duration{d90}, in)
	}
}

// benchField measures decoding in into a field of the type of d, or encoding such a field if in is nil.
func benchField[T any](b *testing.B, d T, in []byte) {
	b.Helper()

	if in == nil {
		benchMarshal(b, field[T]{d})
		return
	}

	benchUnmarshal[field[T]](b, in)
}

// options keeps results of With alive, so that the calls are not optimized away.
var (
	options      json.Options
	marshalers   *json.Marshalers
	unmarshalers *json.Unmarshalers
)

func BenchmarkOptions(b *testing.B) {
	b.Run("With", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			options = With(ISO8601)
		}
	})

	b.Run("With_AlsoAccept_AcceptDays", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			options = With(ISO8601, AlsoAccept(Nano, Units), AcceptDays())
		}
	})

	b.Run("Marshalers", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			marshalers = Marshalers(ISO8601)
		}
	})

	b.Run("Unmarshalers", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			unmarshalers = Unmarshalers(ISO8601, AlsoAccept(Nano))
		}
	})
}

func BenchmarkMarshal(b *testing.B) {
	v := field[time.Duration]{d90}

	b.Run("baseline/int64", func(b *testing.B) { benchMarshal(b, field[int64]{int64(d90)}) })
	b.Run("baseline/nano_option", func(b *testing.B) { benchMarshal(b, v, nanoOption) })
	b.Run("baseline/nano_funcs", func(b *testing.B) { benchMarshal(b, v, nanoFuncs) })
	b.Run("baseline/text_methods", func(b *testing.B) { benchMarshal(b, field[textDuration]{textDuration(d90)}) })

	for _, f := range Formats() {
		opts := With(f)

		b.Run(string(f)+"/options", func(b *testing.B) { benchMarshal(b, v, opts) })
		b.Run(string(f)+"/wrapper", func(b *testing.B) { benchWrapper(b, f, nil) })

		b.Run(string(f)+"/With_per_call", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := json.Marshal(&v, With(f)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkMarshalWrite(b *testing.B) {
	v := field[time.Duration]{d90}

	b.Run("baseline/int64", func(b *testing.B) { benchMarshalWrite(b, field[int64]{int64(d90)}) })
	b.Run("baseline/nano_option", func(b *testing.B) { benchMarshalWrite(b, v, nanoOption) })
	b.Run("baseline/nano_funcs", func(b *testing.B) { benchMarshalWrite(b, v, nanoFuncs) })
	b.Run("baseline/text_methods", func(b *testing.B) { benchMarshalWrite(b, field[textDuration]{textDuration(d90)}) })

	for _, f := range Formats() {
		opts := With(f)
		b.Run(string(f)+"/options", func(b *testing.B) { benchMarshalWrite(b, v, opts) })
	}
}

func BenchmarkUnmarshal(b *testing.B) {
	v := field[time.Duration]{d90}
	nanos := mustMarshal(b, v, nanoOption)

	b.Run("baseline/int64", func(b *testing.B) { benchUnmarshal[field[int64]](b, nanos) })
	b.Run("baseline/nano_option", func(b *testing.B) { benchUnmarshal[field[time.Duration]](b, nanos, nanoOption) })
	b.Run("baseline/nano_funcs", func(b *testing.B) { benchUnmarshal[field[time.Duration]](b, nanos, nanoFuncs) })
	b.Run("baseline/text_methods", func(b *testing.B) {
		benchUnmarshal[field[textDuration]](b, mustMarshal(b, v, With(Units)))
	})

	for _, f := range Formats() {
		opts := With(f)
		in := mustMarshal(b, v, opts)

		b.Run(string(f)+"/options", func(b *testing.B) { benchUnmarshal[field[time.Duration]](b, in, opts) })
		b.Run(string(f)+"/wrapper", func(b *testing.B) { benchWrapper(b, f, in) })

		b.Run(string(f)+"/With_per_call", func(b *testing.B) {
			b.ReportAllocs()

			var got field[time.Duration]
			for b.Loop() {
				if err := json.Unmarshal(in, &got, With(f)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkUnmarshalAlsoAccept measures what a second accepted format costs.
func BenchmarkUnmarshalAlsoAccept(b *testing.B) {
	v := field[time.Duration]{d90}
	iso, units, nanos := mustMarshal(b, v, With(ISO8601)), mustMarshal(b, v, With(Units)), mustMarshal(b, v, nanoOption)

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

// benchRejected measures json.Unmarshal of in into a T, which must fail, and with message also reading the error.
func benchRejected[T any](b *testing.B, in []byte, message bool, opts ...json.Options) {
	b.Helper()
	b.ReportAllocs()

	var got T
	for b.Loop() {
		err := json.Unmarshal(in, &got, opts...)
		if err == nil {
			b.Fatal("no error")
		}

		if message {
			formatted = err.Error()
		}
	}
}

// BenchmarkUnmarshalInvalid measures a duration that is rejected. The error builds its message when asked,
// so there is one benchmark for a caller who only checks the error and one for a caller who reads it.
func BenchmarkUnmarshalInvalid(b *testing.B) {
	in := []byte(`{"D":"1h30m"}`)
	opts := With(ISO8601)

	b.Run("options/checked", func(b *testing.B) { benchRejected[field[time.Duration]](b, in, false, opts) })
	b.Run("options/read", func(b *testing.B) { benchRejected[field[time.Duration]](b, in, true, opts) })
	b.Run("baseline/int64/checked", func(b *testing.B) { benchRejected[field[int64]](b, in, false) })
	b.Run("baseline/int64/read", func(b *testing.B) { benchRejected[field[int64]](b, in, true) })
}

// BenchmarkSlice measures a thousand durations in one value, to show the cost of each and that allocations do not
// grow with their number.
func BenchmarkSlice(b *testing.B) {
	const n = 1000

	durations := manyDurations(n)
	ints := make([]int64, n)
	for i, d := range durations {
		ints[i] = int64(d)
	}

	nanos := mustMarshal(b, ints)

	b.Run("Marshal/baseline/int64", func(b *testing.B) { benchMarshal(b, ints) })
	b.Run("Marshal/baseline/nano_option", func(b *testing.B) { benchMarshal(b, durations, nanoOption) })
	b.Run("Marshal/baseline/nano_funcs", func(b *testing.B) { benchMarshal(b, durations, nanoFuncs) })
	b.Run("Unmarshal/baseline/int64", func(b *testing.B) { benchUnmarshal[[]int64](b, nanos) })
	b.Run("Unmarshal/baseline/nano_option", func(b *testing.B) {
		benchUnmarshal[[]time.Duration](b, nanos, nanoOption)
	})
	b.Run("Unmarshal/baseline/nano_funcs", func(b *testing.B) {
		benchUnmarshal[[]time.Duration](b, nanos, nanoFuncs)
	})

	for _, f := range Formats() {
		opts := With(f)
		in := mustMarshal(b, durations, opts)

		b.Run("Marshal/"+string(f)+"/options", func(b *testing.B) { benchMarshal(b, durations, opts) })
		b.Run("Unmarshal/"+string(f)+"/options", func(b *testing.B) { benchUnmarshal[[]time.Duration](b, in, opts) })
	}
}

// BenchmarkConfig measures a value with other fields than durations, to show what share the durations have.
func BenchmarkConfig(b *testing.B) {
	durations := configOf(func(d time.Duration) time.Duration { return d })
	ints := configOf(func(d time.Duration) int64 { return int64(d) })
	nanos := mustMarshal(b, ints)

	b.Run("Marshal/baseline/int64", func(b *testing.B) { benchMarshal(b, ints) })
	b.Run("Marshal/baseline/nano_option", func(b *testing.B) { benchMarshal(b, durations, nanoOption) })
	b.Run("Marshal/baseline/nano_funcs", func(b *testing.B) { benchMarshal(b, durations, nanoFuncs) })
	b.Run("Unmarshal/baseline/int64", func(b *testing.B) { benchUnmarshal[config[int64]](b, nanos) })
	b.Run("Unmarshal/baseline/nano_option", func(b *testing.B) {
		benchUnmarshal[config[time.Duration]](b, nanos, nanoOption)
	})
	b.Run("Unmarshal/baseline/nano_funcs", func(b *testing.B) {
		benchUnmarshal[config[time.Duration]](b, nanos, nanoFuncs)
	})

	for _, f := range Formats() {
		opts := With(f)
		in := mustMarshal(b, durations, opts)

		b.Run("Marshal/"+string(f)+"/options", func(b *testing.B) { benchMarshal(b, durations, opts) })
		b.Run("Marshal/"+string(f)+"/wrapper", func(b *testing.B) { benchWrapperConfig(b, f, nil) })
		b.Run("Unmarshal/"+string(f)+"/options", func(b *testing.B) {
			benchUnmarshal[config[time.Duration]](b, in, opts)
		})
		b.Run("Unmarshal/"+string(f)+"/wrapper", func(b *testing.B) { benchWrapperConfig(b, f, in) })
	}
}

func BenchmarkFormat(b *testing.B) {
	b.Run("baseline/Duration.String", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			formatted = d90.String()
		}
	})

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
	b.Run("baseline/time.ParseDuration", func(b *testing.B) {
		s := d90.String()

		b.ReportAllocs()
		for b.Loop() {
			if _, err := time.ParseDuration(s); err != nil {
				b.Fatal(err)
			}
		}
	})

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
