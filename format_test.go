package jsonduration

import (
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"testing"
	"time"
)

const (
	maxD = time.Duration(math.MaxInt64)
	minD = time.Duration(math.MinInt64)
	d90  = 90*time.Minute + 500*time.Millisecond
)

func TestFormat(t *testing.T) {
	tests := []struct {
		f    Format
		d    time.Duration
		want string
	}{
		{Units, 0, "0s"},
		{Units, d90, "1h30m0.5s"},
		{Units, 1500 * time.Nanosecond, "1.5µs"},
		{Units, minD, "-2562047h47m16.854775808s"},

		{Sec, 0, "0"},
		{Sec, time.Nanosecond, "0.000000001"},
		{Sec, -time.Nanosecond, "-0.000000001"},
		{Sec, d90, "5400.5"},
		{Sec, maxD, "9223372036.854775807"},
		{Sec, minD, "-9223372036.854775808"},
		{Milli, d90, "5400500"},
		{Milli, 1500 * time.Microsecond, "1.5"},
		{Micro, d90, "5400500000"},
		{Nano, d90, "5400500000000"},
		{Nano, maxD, "9223372036854775807"},
		{Nano, minD, "-9223372036854775808"},

		{ISO8601, 0, "PT0S"},
		{ISO8601, time.Nanosecond, "PT0.000000001S"},
		{ISO8601, d90, "PT1H30M0.5S"},
		{ISO8601, time.Hour + time.Second, "PT1H1S"},
		{ISO8601, -time.Minute, "-PT1M"},
		{ISO8601, maxD, "PT2562047H47M16.854775807S"},
		{ISO8601, minD, "-PT2562047H47M16.854775808S"},
	}

	for _, tt := range tests {
		t.Run(string(tt.f)+"/"+tt.want, func(t *testing.T) {
			got := tt.f.Format(tt.d)
			if got != tt.want {
				t.Fatalf("%s.Format(%d) = %q, want %q", tt.f, int64(tt.d), got, tt.want)
			}

			if back, err := tt.f.Parse(got); err != nil || back != tt.d {
				t.Errorf("%s.Parse(%q) = %v, %v, want %v", tt.f, got, back, err, tt.d)
			}
		})
	}
}

// TestLongestOutput computes the longest output from the extremes, where every format writes the most digits,
// and requires maxPossibleOutputLength to match it exactly. FuzzRoundTrip checks that no other duration writes more.
func TestLongestOutput(t *testing.T) {
	extremes := []time.Duration{maxD, minD}

	longest := 0
	for _, f := range Formats() {
		for _, d := range extremes {
			longest = max(longest, len(f.Format(d)))
		}
	}

	if longest != maxPossibleOutputLength {
		t.Errorf("the longest output has %d bytes, but longestOutput = %d", longest, maxPossibleOutputLength)
	}
}

type parseTest struct {
	in      string
	want    time.Duration
	wantErr error
}

func testParse(t *testing.T, f Format, tests []parseTest) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := f.Parse(tt.in)

			switch {
			case tt.wantErr != nil && (!errors.Is(err, tt.wantErr) || got != 0):
				t.Errorf("%s.Parse(%q) = %v, %v, want 0 and an error caused by %v", f, tt.in, got, err, tt.wantErr)
			case tt.wantErr == nil && (err != nil || got != tt.want):
				t.Errorf("%s.Parse(%q) = %v, %v, want %v", f, tt.in, got, err, tt.want)
			}
		})
	}
}

func TestFormatTextRoundTrip(t *testing.T) {
	for _, f := range Formats() {
		b, err := f.MarshalText()
		if err != nil || string(b) != string(f) {
			t.Errorf("%s.MarshalText() = %s, %v", f, b, err)
		}

		var g Format
		if err := g.UnmarshalText(b); err != nil || g != f {
			t.Errorf("UnmarshalText(%q) = %q, %v, want %q", b, g, err, f)
		}
	}
}

func TestFormatMarshalTextInvalid(t *testing.T) {
	bogus := Format("bogus")

	if _, err := bogus.MarshalText(); err == nil {
		t.Error(`Format("bogus").MarshalText() succeeded, want an error`)
	}
}

func TestFormatUnmarshalTextRejectsOtherNames(t *testing.T) {
	const names = "units, sec, milli, micro, nano, iso8601"
	inputs := []string{"bogus", "ISO8601", "Nano", " sec", ""}

	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			f := Sec
			err := f.UnmarshalText([]byte(in))

			want := fmt.Sprintf("unknown format %q, want one of %s", in, names)
			if err == nil || err.Error() != want {
				t.Errorf("UnmarshalText(%q) error = %v, want %s", in, err, want)
			}

			if f != Sec {
				t.Errorf("UnmarshalText(%q) changed the format to %q", in, f)
			}
		})
	}
}

func TestFormatFromFlag(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var f Format
	fs.TextVar(&f, "format", ISO8601, "duration format")

	args := []string{"-format=nano"}
	if err := fs.Parse(args); err != nil || f != Nano {
		t.Errorf("-format=nano gives %q, %v, want nano", f, err)
	}

	args = []string{"-format=Nano"}
	if err := fs.Parse(args); err == nil {
		t.Error("-format=Nano succeeded, want an error")
	}
}

func TestFormatFromJSON(t *testing.T) {
	type config struct {
		Format Format
	}

	var cfg config
	err := json.Unmarshal([]byte(`{"Format":"iso8601"}`), &cfg)
	if err != nil || cfg.Format != ISO8601 {
		t.Errorf(`{"Format":"iso8601"} gives %q, %v, want iso8601`, cfg.Format, err)
	}

	invalid := []string{`{"Format":"ISO8601"}`, `{"Format":"weeks"}`}
	for _, in := range invalid {
		if err := json.Unmarshal([]byte(in), &cfg); err == nil {
			t.Errorf("Unmarshal(%s) succeeded, want an error", in)
		}
	}
}

func TestValid(t *testing.T) {
	for _, f := range Formats() {
		if !f.Valid() {
			t.Errorf("%q.Valid() = false, want true", f)
		}
	}

	invalid := []Format{"", "bogus", "ISO8601", "Iso8601", "iso8601 ", "Nano"}
	for _, f := range invalid {
		if f.Valid() {
			t.Errorf("%q.Valid() = true, want false", f)
		}
	}
}

func FuzzRoundTrip(f *testing.F) {
	seeds := []int64{0, 1, -1, int64(d90), math.MaxInt64, math.MinInt64, 1500}
	for _, n := range seeds {
		f.Add(n)
	}

	f.Fuzz(func(t *testing.T, n int64) {
		d := time.Duration(n)

		for _, fm := range Formats() {
			s := fm.Format(d)
			if got, err := fm.Parse(s); err != nil || got != d {
				t.Fatalf("%s.Parse(%s.Format(%d)) = %d, %v", fm, fm, n, int64(got), err)
			}

			if len(s) > maxPossibleOutputLength {
				t.Fatalf("%s.Format(%d) = %q, longer than longestOutput = %d", fm, n, s, maxPossibleOutputLength)
			}
		}
	})
}

// FuzzParse checks that whatever a format reads, it writes back to the same value,
// and that two formats AlsoAccept can combine never read the same text as different durations.
func FuzzParse(f *testing.F) {
	seeds := []string{
		"1h30m", "5400.5", "PT1H30M0.5S", "1:30:00.5", "1.500s", "-0", "P1D", "PT1.5H",
		"0", "+0", "1.5s", ".5s", "1.000000001s", "0.3s", "-0.000000001s",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		accepted := map[Format]time.Duration{}

		for _, fm := range Formats() {
			d, err := fm.Parse(s)
			if err != nil {
				continue
			}

			accepted[fm] = d

			written := fm.Format(d)
			if again, err := fm.Parse(written); err != nil || again != d {
				t.Fatalf("%s: %q reads as %d, writes as %q, reads back as %d, %v",
					fm, s, int64(d), written, int64(again), err)
			}
		}

		for a, da := range accepted {
			for b, db := range accepted {
				if a < b && (!a.mustCodec().numeric() || !b.mustCodec().numeric()) && da != db {
					t.Fatalf("%q is %d in %s but %d in %s", s, int64(da), a, int64(db), b)
				}
			}
		}
	})
}
