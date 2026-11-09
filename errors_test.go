package jsonduration

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseUnitsErrorMessages(t *testing.T) {
	tests := map[string]string{
		"1":  `invalid duration "1" in format units: missing unit: invalid syntax`,
		"1x": `invalid duration "1x" in format units: unknown unit "x": invalid syntax`,
	}

	for in, want := range tests {
		if _, err := Units.Parse(in); err == nil || err.Error() != want {
			t.Errorf("Units.Parse(%q) error = %v, want %s", in, err, want)
		}
	}
}

func TestInvalidFormatPanics(t *testing.T) {
	const bogus = Format("bogus")

	tests := []struct {
		name string
		fn   func()
		want string
	}{
		{"With", func() { With(bogus) }, "invalid format"},
		{"Marshalers", func() { Marshalers(bogus) }, "invalid format"},
		{"Unmarshalers", func() { Unmarshalers(bogus) }, "invalid format"},
		{"Format", func() { bogus.Format(time.Second) }, "invalid format"},
		{"Parse", func() { _, _ = bogus.Parse("1s") }, "invalid format"},
		{"AlsoAccept invalid", func() { With(ISO8601, AlsoAccept(bogus)) }, "invalid format"},
		{"AlsoAccept a second numeric", func() { With(Nano, AlsoAccept(Sec)) }, "both nano and sec"},
		{"AlsoAccept two numeric", func() { Unmarshalers(ISO8601, AlsoAccept(Milli, Micro)) }, "both milli and micro"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				msg := fmt.Sprint(recover())
				if !strings.Contains(msg, tt.want) {
					t.Errorf("panic %q, want one containing %q", msg, tt.want)
				}
			}()
			tt.fn()
		})
	}
}

func TestErrInvalidDuration(t *testing.T) {
	type config struct {
		Timeout ISO8601Duration
	}

	unknown := Format("bogus")
	strict := With(ISO8601)
	both := With(ISO8601, AlsoAccept(Nano))

	invalid := map[string]func() error{
		"Units.Parse":     func() error { _, err := Units.Parse("1x"); return err },
		"Sec.Parse":       func() error { _, err := Sec.Parse("1e3"); return err },
		"ISO8601.Parse":   func() error { _, err := ISO8601.Parse("PT1X"); return err },
		"calendar units":  func() error { _, err := ISO8601.Parse("P1D"); return err },
		"out of range":    func() error { _, err := Nano.Parse("99999999999999999999"); return err },
		"Unmarshal":       func() error { var d time.Duration; return json.Unmarshal([]byte(`"1h"`), &d, strict) },
		"wrong JSON kind": func() error { var d time.Duration; return json.Unmarshal([]byte(`90`), &d, strict) },
		"AlsoAccept":      func() error { var d time.Duration; return json.Unmarshal([]byte(`"1h"`), &d, both) },
		"wrapper type":    func() error { var c config; return json.Unmarshal([]byte(`{"Timeout":"1h"}`), &c) },
	}

	shape := regexp.MustCompile(`^invalid duration ".*" in format [a-z0-9]+( or [a-z0-9]+)*: .+$`)

	for name, fn := range invalid {
		err := fn()
		if !errors.Is(err, ErrInvalidDuration) {
			t.Errorf("%s: error %v does not match ErrInvalidDuration", name, err)
			continue
		}

		ours := err
		if semantic, ok := errors.AsType[*json.SemanticError](err); ok {
			ours = semantic.Err
		}

		if !shape.MatchString(ours.Error()) {
			t.Errorf("%s: message %q does not have the shape %s", name, ours, shape)
		}
	}

	wrongType := map[string]bool{"wrong JSON kind": true}
	for name, fn := range invalid {
		if got := errors.Is(fn(), ErrWrongType); got != wrongType[name] {
			t.Errorf("%s: errors.Is(err, ErrWrongType) = %v, want %v", name, got, wrongType[name])
		}
	}

	other := map[string]func() error{
		"invalid Format": func() error { _, err := unknown.MarshalText(); return err },
		"unknown name":   func() error { var f Format; return f.UnmarshalText([]byte("weeks")) },
		"JSON syntax":    func() error { var d time.Duration; return json.Unmarshal([]byte(`"PT1H`), &d, strict) },
	}

	for name, fn := range other {
		if err := fn(); err == nil || errors.Is(err, ErrInvalidDuration) {
			t.Errorf("%s: error %v, want one that does not match ErrInvalidDuration", name, err)
		}
	}

	t.Run("quotes 64 bytes in full", func(t *testing.T) {
		exact := strings.Repeat("x", 64)
		_, err := Units.Parse(exact)

		want := `invalid duration "` + exact + `" in format units: invalid syntax`
		if err == nil || err.Error() != want {
			t.Errorf("error = %v, want %s", err, want)
		}
	})

	t.Run("quotes at most 64 bytes of the input", func(t *testing.T) {
		long := strings.Repeat("x", 100)
		_, err := Units.Parse(long)

		want := `invalid duration "` + long[:64] + `"... in format units: invalid syntax`
		if err == nil || err.Error() != want {
			t.Errorf("error = %v, want %s", err, want)
		}
	})

	t.Run("does not cut a character in two", func(t *testing.T) {
		// Each character straddles byte 64: é has 2 bytes, € has 3 and 😀 has 4.
		tests := []struct {
			prefix int
			char   string
			kept   int
		}{
			{64, "é", 64}, {63, "é", 63},
			{63, "€", 63}, {62, "€", 62},
			{63, "😀", 63}, {62, "😀", 62}, {61, "😀", 61},
			{60, "😀", 64}, {62, "é", 64}, {61, "€", 64},
		}

		for _, tt := range tests {
			in := strings.Repeat("x", tt.prefix) + tt.char + strings.Repeat("x", 10)
			_, err := Units.Parse(in)

			want := `invalid duration "` + in[:tt.kept] + `"... in format units: invalid syntax`
			if err == nil || err.Error() != want {
				t.Errorf("%d bytes and then %s: error = %v, want %s", tt.prefix, tt.char, err, want)
			}
		}
	})

	t.Run("cuts text that is not UTF-8 at 64 bytes", func(t *testing.T) {
		in := strings.Repeat("\x80", 100)
		_, err := Units.Parse(in)

		want := "invalid duration " + strconv.Quote(in[:64]) + "... in format units: invalid syntax"
		if err == nil || err.Error() != want {
			t.Errorf("error = %v, want %s", err, want)
		}
	})

	t.Run("quotes at most 64 bytes of a unit or format name", func(t *testing.T) {
		long := strings.Repeat("x", 1000)

		_, err := Units.Parse("1" + long)
		if err == nil || len(err.Error()) > 256 {
			t.Errorf("Units.Parse error has %d bytes, want at most 256", len(err.Error()))
		}

		var f Format
		err = f.UnmarshalText([]byte(long))
		if err == nil || len(err.Error()) > 256 {
			t.Errorf("UnmarshalText error has %d bytes, want at most 256", len(err.Error()))
		}
	})

	t.Run("keeps the message and the wrapped errors", func(t *testing.T) {
		_, err := ISO8601.Parse("P1D")
		const want = `invalid duration "P1D" in format iso8601: calendar units have no fixed length`
		if err.Error() != want || !errors.Is(err, ErrCalendarUnits) {
			t.Errorf("error = %q, want %q wrapping ErrCalendarUnits", err, want)
		}
	})
}
