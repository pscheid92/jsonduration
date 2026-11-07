package jsonduration

import (
	"math/big"
	"math/rand/v2"
	"strconv"
	"testing"
	"time"
)

func TestParseISO8601(t *testing.T) {
	tests := []parseTest{
		{"PT1H1S", time.Hour + time.Second, nil},
		{"pt1h30m", 90 * time.Minute, nil},
		{"+PT1S", time.Second, nil},
		{"-PT1.5S", -1500 * time.Millisecond, nil},
		{"PT0,5S", 500 * time.Millisecond, nil},
		{"PT1.5H", 90 * time.Minute, nil},
		{"PT007S", 7 * time.Second, nil},
		{"-PT0S", 0, nil},
		{"P1D", 0, ErrCalendarUnits},
		{"P1W", 0, ErrCalendarUnits},
		{"P1Y2M", 0, ErrCalendarUnits},
		{"P1DT1H", 0, ErrCalendarUnits},
		{"", 0, strconv.ErrSyntax},
		{"P", 0, strconv.ErrSyntax},
		{"PT", 0, strconv.ErrSyntax},
		{"T1S", 0, strconv.ErrSyntax},
		{"PT-1S", 0, strconv.ErrSyntax},
		{"PT1S1M", 0, strconv.ErrSyntax},
		{"PT1.5H30M", 0, strconv.ErrSyntax},
		{"PT1.S", 0, strconv.ErrSyntax},
		{"PT1.5e1H", 0, strconv.ErrSyntax},
		{"P1.5D", 0, ErrCalendarUnits},
		{"P0.5W", 0, ErrCalendarUnits},
		{"P1.5Y", 0, ErrCalendarUnits},
		{"PT1X", 0, strconv.ErrSyntax},
		{"PT2562048H", 0, strconv.ErrRange},
		{"PT5124095577H", 0, strconv.ErrRange},
		{"P1D1", 0, strconv.ErrSyntax},
		{"PT1H1", 0, strconv.ErrSyntax},
		{"-PT2562047H47M16.854775809S", 0, strconv.ErrRange},
	}

	testParse(t, ISO8601, tests)
}

func TestISO8601FractionsAreTruncated(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want time.Duration
	}{
		{"0.72ns of an hour", "PT0.0000000000002H", 0},
		{"1.2ns of a minute", "PT0.00000000002M", 1},
		{"just below 1/36 hour", "PT0.02777777777777777777H", 100*time.Second - 1},
		{"just above 1/36 hour", "PT0.02777777777777777778H", 100 * time.Second},
		{"just below 1/6 minute", "PT0.16666666666666666666M", 10*time.Second - 1},
		{"just above 1/6 minute", "PT0.16666666666666666667M", 10 * time.Second},
		{"minus 0.72ns", "-PT0.0000000000002H", 0},
		{"minus 1.2ns", "-PT0.00000000002M", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := ISO8601.Parse(tt.in); err != nil || got != tt.want {
				t.Errorf("ISO8601.Parse(%q) = %d, %v, want %d", tt.in, int64(got), err, int64(tt.want))
			}
		})
	}
}

func TestISO8601FractionsMatchExactArithmetic(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	units := map[string]time.Duration{"H": time.Hour, "M": time.Minute, "S": time.Second}

	for range 20000 {
		digits := make([]byte, 1+r.IntN(25))
		for i := range digits {
			digits[i] = byte('0' + r.IntN(10))
		}

		for designator, unit := range units {
			in := "PT0." + string(digits) + designator
			want := truncatedFraction(string(digits), unit)

			if got, err := ISO8601.Parse(in); err != nil || got != want {
				t.Fatalf("ISO8601.Parse(%q) = %d, %v, want %d", in, int64(got), err, int64(want))
			}
		}
	}
}

// truncatedFraction returns ⌊0.digits × unit⌋, computed with math/big.
func truncatedFraction(digits string, unit time.Duration) time.Duration {
	fraction, _ := new(big.Rat).SetString("0." + digits)
	scale := new(big.Rat).SetInt64(int64(unit))
	exact := fraction.Mul(fraction, scale)

	truncated := new(big.Int).Quo(exact.Num(), exact.Denom())
	return time.Duration(truncated.Int64())
}
