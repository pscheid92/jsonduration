package jsonduration

import (
	"bytes"
	"encoding/json/v2"
	"sync"
	"testing"
	"time"
)

// TestSameArgumentsShareOptions checks which calls get the same values: those that write the same format and read the
// same formats. It compares the unmarshalers, since they are pointers and the options are not comparable.
func TestSameArgumentsShareOptions(t *testing.T) {
	base := Unmarshalers(ISO8601, AlsoAccept(Nano, Units))

	same := map[string]*json.Unmarshalers{
		"again":           Unmarshalers(ISO8601, AlsoAccept(Nano, Units)),
		"other order":     Unmarshalers(ISO8601, AlsoAccept(Units, Nano)),
		"a format twice":  Unmarshalers(ISO8601, AlsoAccept(Nano, Units, ISO8601, Nano)),
		"two options":     Unmarshalers(ISO8601, AlsoAccept(), AlsoAccept(Nano), AlsoAccept(Units)),
		"a nil option":    Unmarshalers(ISO8601, nil, AlsoAccept(Nano, Units)),
		"the written one": Unmarshalers(ISO8601, AlsoAccept(ISO8601), AlsoAccept(Nano, Units)),
	}
	for name, got := range same {
		if got != base {
			t.Errorf("%s: got other unmarshalers, want the shared ones", name)
		}
	}

	different := map[string]*json.Unmarshalers{
		"other written format": Unmarshalers(Nano, AlsoAccept(ISO8601, Units)),
		"no other formats":     Unmarshalers(ISO8601),
		"fewer formats":        Unmarshalers(ISO8601, AlsoAccept(Nano)),
		"other format":         Unmarshalers(ISO8601, AlsoAccept(Sec, Units)),
		"with days":            Unmarshalers(ISO8601, AlsoAccept(Nano, Units), AcceptDays()),
	}
	for name, got := range different {
		if got == base {
			t.Errorf("%s: got the unmarshalers of ISO8601 with Nano and Units", name)
		}
	}

	for _, f := range Formats() {
		first, second := Marshalers(f), Marshalers(f)
		if first != second {
			t.Errorf("Marshalers(%s) differ between calls", f)
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

// TestWithDoesNotAllocate checks the promise in the documentation of With: asking again costs no allocation,
// with decoding options or without, so that decoding with options from a call allocates as little as with kept ones,
// which is not at all.
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
			{"With options", 0, func() { _ = With(f, AlsoAccept(Units, ISO8601), AcceptDays()) }},
			{"Unmarshalers with options", 0, func() { _ = Unmarshalers(f, AlsoAccept(Units, ISO8601), AcceptDays()) }},
			{"Unmarshal", 0, func() { _ = json.Unmarshal(in, &d, With(f)) }},
			{"Unmarshal with options", 0, func() { _ = json.Unmarshal(in, &d, With(f, AlsoAccept(Units, ISO8601))) }},
		}

		for _, tt := range tests {
			if got := testing.AllocsPerRun(100, tt.fn); got != tt.want {
				t.Errorf("%s: %s allocs = %v, want %v", f, tt.name, got, tt.want)
			}
		}
	}
}

// TestInvalidArgumentsAreNotStored checks that a call that panics leaves nothing behind,
// so that the shared values cannot grow without bound, and that it panics again the next time.
func TestInvalidArgumentsAreNotStored(t *testing.T) {
	count := func() int {
		n := 0
		for i := range sharedByArgs {
			for j := range sharedByArgs[i] {
				for k := range sharedByArgs[i][j] {
					if sharedByArgs[i][j][k].Load() != nil {
						n++
					}
				}
			}
		}

		return n
	}

	// Build the valid part first: afterwards, an invalid call must not find it.
	With(ISO8601)
	before := count()

	invalid := map[string]func(){
		"unknown format":         func() { With("bogus") },
		"empty format":           func() { With("") },
		"unknown accepted":       func() { With(ISO8601, AlsoAccept("bogus")) },
		"empty accepted":         func() { With(ISO8601, AlsoAccept("")) },
		"two numeric":            func() { With(Nano, AlsoAccept(Sec)) },
		"unknown after a second": func() { With(Nano, AlsoAccept(Sec, "bogus")) },
	}

	for name, fn := range invalid {
		for range 2 {
			func() {
				defer func() {
					if recover() == nil {
						t.Errorf("%s: no panic", name)
					}
				}()
				fn()
			}()
		}
	}

	if after := count(); after != before {
		t.Errorf("%d shared values after invalid calls, want %d as before", after, before)
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
}

// TestConcurrentFirstCalls only finds a data race when run with -race, as CI does.
// Every goroutine must get the same values, whichever built them.
func TestConcurrentFirstCalls(t *testing.T) {
	// No other test asks for this set of arguments, so these calls are the first.
	args := func() *json.Unmarshalers { return Unmarshalers(Micro, AlsoAccept(Units, ISO8601), AcceptDays()) }

	got := make([]*json.Unmarshalers, 16)

	var wg sync.WaitGroup
	for i := range got {
		wg.Go(func() { got[i] = args() })
	}

	wg.Wait()

	for i, u := range got {
		if u != got[0] {
			t.Errorf("goroutine %d got other unmarshalers than goroutine 0", i)
		}
	}
}
