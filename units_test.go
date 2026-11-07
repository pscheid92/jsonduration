package jsonduration

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseUnits(t *testing.T) {
	tests := []parseTest{
		{"1h30m", 90 * time.Minute, nil},
		{"1us", time.Microsecond, nil},
		{"1\u00b5s", time.Microsecond, nil},
		{"1\u03bcs", time.Microsecond, nil},
		{"+1.5h", 90 * time.Minute, nil},
		{".5s", 500 * time.Millisecond, nil},
		{"5.s", 5 * time.Second, nil},
		{"01s", time.Second, nil},
		{"1h.5m", time.Hour + 30*time.Second, nil},
		{"0", 0, nil},
		{"-0", 0, nil},
		{"0.0000000019s", time.Nanosecond, nil},
		{"0.0000000000002h", 0, nil},
		{"-9223372036854775808ns", minD, nil},
		{"", 0, strconv.ErrSyntax},
		{"-", 0, strconv.ErrSyntax},
		{"1", 0, strconv.ErrSyntax},
		{"1x", 0, strconv.ErrSyntax},
		{".s", 0, strconv.ErrSyntax},
		{" 1s", 0, strconv.ErrSyntax},
		{"1h-1m", 0, strconv.ErrSyntax},
		{"PT1H", 0, strconv.ErrSyntax},
		{"2562048h", 0, strconv.ErrRange},
		{"5124095577h", 0, strconv.ErrRange},
		{"9223372036854775807ns9223372036854775807ns9223372036854775807ns", 0, strconv.ErrRange},
		{"9223372036854775808ns", 0, strconv.ErrRange},
		{"99999999999999999999999ns", 0, strconv.ErrRange},
		{"2562047h47m16.854775807s1ns", 0, strconv.ErrRange},
	}

	testParse(t, Units, tests)
}

// FuzzUnitsMatchesParseDuration checks that Units reads exactly what time.ParseDuration reads, with the same value.
// time.ParseDuration computes fractions through float64, so a component with a fraction may come out 1ns apart,
// which can also push a value just past the range of a Duration.
func FuzzUnitsMatchesParseDuration(f *testing.F) {
	seeds := []string{
		"1h30m", "1.5us", "1\u00b5s", "1\u03bcs", "0", "-0", "+.5s", "5.s", "1h1h", "0.1h0.1h0.1h",
		"9223372036854775807ns", "-9223372036854775808ns", "9223372036854775808ns",
		"2562047h47m16.854775807s", "2562047h47m16.854775808s", "-2562047h47m16.854775808s",
		"1.0000000000000000000001s", "0.123456789012345h", ".s", "1", "", "1x", "1h-1m",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		want, wantErr := time.ParseDuration(s)
		got, err := Units.Parse(s)

		slack := uint64(strings.Count(s, "."))
		near := func(a, b time.Duration) bool {
			if a < b {
				a, b = b, a
			}

			return uint64(a)-uint64(b) <= slack
		}

		nearLimit := func(d time.Duration) bool {
			return near(d, maxD) || near(d, minD)
		}

		switch {
		case err == nil && wantErr == nil && !near(got, want):
			t.Fatalf("Units.Parse(%q) = %d, time.ParseDuration = %d", s, int64(got), int64(want))
		case err == nil && wantErr != nil && !nearLimit(got):
			t.Fatalf("Units.Parse(%q) = %d, but time.ParseDuration fails: %v", s, int64(got), wantErr)
		case err != nil && wantErr == nil && !nearLimit(want):
			t.Fatalf("Units.Parse(%q) fails: %v, but time.ParseDuration = %d", s, err, int64(want))
		}

		if err != nil && !errors.Is(err, strconv.ErrSyntax) && !errors.Is(err, strconv.ErrRange) {
			t.Fatalf("Units.Parse(%q) error %v wraps neither ErrSyntax nor ErrRange", s, err)
		}
	})
}
