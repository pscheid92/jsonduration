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
				// Format and Parse allocate once more than needed: their buffer escapes through the codec interface.
				{"Format", 2, func() error { formatted = f.Format(d90); return nil }},
				{"Parse", 1, func() error { _, err := f.Parse(text); return err }},
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

func BenchmarkMarshal(b *testing.B) {
	v := object{D: d90}

	for _, f := range Formats() {
		b.Run(string(f), func(b *testing.B) {
			opts := With(f)

			b.ReportAllocs()
			for b.Loop() {
				if _, err := json.Marshal(v, opts); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkMarshalInt64Baseline(b *testing.B) {
	type baseline struct {
		D int64
	}

	v := baseline{D: int64(d90)}

	b.ReportAllocs()
	for b.Loop() {
		if _, err := json.Marshal(v); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshal(b *testing.B) {
	v := object{D: d90}

	for _, f := range Formats() {
		opts := With(f)

		in, err := json.Marshal(v, opts)
		if err != nil {
			b.Fatal(err)
		}

		b.Run(string(f), func(b *testing.B) {
			var got object

			b.ReportAllocs()
			for b.Loop() {
				if err := json.Unmarshal(in, &got, opts); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkUnmarshalInt64Baseline(b *testing.B) {
	type baseline struct {
		D int64
	}

	in := []byte(`{"D":5400500000000}`)
	var got baseline

	b.ReportAllocs()
	for b.Loop() {
		if err := json.Unmarshal(in, &got); err != nil {
			b.Fatal(err)
		}
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
