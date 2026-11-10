package jsonduration

import (
	"bytes"
	"encoding/json/v2"
	"testing"
	"time"
)

// TestFormatSharesOptions checks which calls get the same values: those that write the same format and have no
// option that changes anything. It compares the unmarshalers, since they are pointers and the options are not
// comparable.
func TestFormatSharesOptions(t *testing.T) {
	for _, f := range Formats() {
		if first, second := Marshalers(f), Marshalers(f); first != second {
			t.Errorf("Marshalers(%s) differ between calls", f)
		}

		base := Unmarshalers(f)

		same := map[string]*json.Unmarshalers{
			"again":           Unmarshalers(f),
			"a nil option":    Unmarshalers(f, nil),
			"an empty option": Unmarshalers(f, AlsoAccept()),
		}
		for name, got := range same {
			if got != base {
				t.Errorf("%s, %s: got other unmarshalers, want the shared ones", f, name)
			}
		}

		different := map[string]*json.Unmarshalers{
			"another format": Unmarshalers(f, AlsoAccept(Units, ISO8601)),
			"with days":      Unmarshalers(f, AcceptDays()),
		}
		for name, got := range different {
			if got == base {
				t.Errorf("%s, %s: got the unmarshalers of the format alone", f, name)
			}
		}
	}
}

// TestOptionsDoNotChangeMarshalers checks that decoding options leave the writing half alone.
func TestOptionsDoNotChangeMarshalers(t *testing.T) {
	want, err := json.Marshal(d90, With(ISO8601))
	if err != nil {
		t.Fatal(err)
	}

	got, err := json.Marshal(d90, With(ISO8601, AlsoAccept(Nano, Units), AcceptDays()))
	if err != nil || !bytes.Equal(got, want) {
		t.Errorf("Marshal with decoding options = %s, %v, want %s", got, err, want)
	}
}

// TestWithDoesNotAllocate checks the promise in the documentation of With: without decoding options, asking again
// costs no allocation, so that decoding with options from a call allocates as little as with kept ones, which is not
// at all.
func TestWithDoesNotAllocate(t *testing.T) {
	for _, f := range Formats() {
		in, err := json.Marshal(d90, With(f))
		if err != nil {
			t.Fatal(err)
		}

		var d time.Duration
		tests := []struct {
			name string
			want float64
			fn   func()
		}{
			{"With", 0, func() { _ = With(f) }},
			{"Marshalers", 0, func() { _ = Marshalers(f) }},
			{"Unmarshalers", 0, func() { _ = Unmarshalers(f) }},
			{"With a nil option", 0, func() { _ = With(f, nil) }},
			{"Unmarshal", 0, func() { _ = json.Unmarshal(in, &d, With(f)) }},
		}

		for _, tt := range tests {
			if got := testing.AllocsPerRun(100, tt.fn); got != tt.want {
				t.Errorf("%s: %s allocs = %v, want %v", f, tt.name, got, tt.want)
			}
		}
	}
}

// TestOptionValueIsReusable checks that an option used in several calls is not changed by the options after it.
func TestOptionValueIsReusable(t *testing.T) {
	units := AlsoAccept(Units)

	both := With(Nano, units, AlsoAccept(ISO8601))
	onlyUnits := With(Sec, units)

	const iso, inUnits = `"PT1H30M0.5S"`, `"1h30m0.5s"`

	var d time.Duration
	if err := json.Unmarshal([]byte(iso), &d, both); err != nil || d != d90 {
		t.Errorf("Unmarshal(%s) with both = %v, %v, want %v", iso, d, err, d90)
	}

	if err := json.Unmarshal([]byte(iso), &d, onlyUnits); err == nil {
		t.Errorf("Unmarshal(%s) with units only = %v, want an error", iso, d)
	}

	d = 0
	if err := json.Unmarshal([]byte(inUnits), &d, onlyUnits); err != nil || d != d90 {
		t.Errorf("Unmarshal(%s) with units only = %v, %v, want %v", inUnits, d, err, d90)
	}

	// The option keeps the formats it was given, also if the caller changes the slice afterwards.
	formats := []Format{Units}
	fromSlice := AlsoAccept(formats...)
	formats[0] = ISO8601

	d = 0
	if err := json.Unmarshal([]byte(inUnits), &d, With(Sec, fromSlice)); err != nil || d != d90 {
		t.Errorf("Unmarshal(%s) after the slice changed = %v, %v, want %v", inUnits, d, err, d90)
	}
}
