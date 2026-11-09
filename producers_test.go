package jsonduration

import (
	"encoding/json/v2"
	"errors"
	"strconv"
	"testing"
	"time"
)

// TestAcceptDaysReadsWhatProducersWrite pins durations exactly as Python's Pydantic 2 and .NET's XmlConvert wrote
// them, checked on 2026-11-08, so that a change that would stop reading them fails here.
func TestAcceptDaysReadsWhatProducersWrite(t *testing.T) {
	opts := With(ISO8601, AcceptDays())

	tests := []struct {
		producer string
		in       string
		want     time.Duration
	}{
		{"Pydantic timedelta(hours=36)", `"P1DT12H"`, 36 * time.Hour},
		{"Pydantic timedelta(hours=-36)", `"-P1DT12H"`, -36 * time.Hour},
		{"Pydantic timedelta(days=7)", `"P7D"`, week},
		{"Pydantic timedelta(days=14)", `"P14D"`, 2 * week},
		{"Pydantic timedelta(minutes=90)", `"PT1H30M"`, 90 * time.Minute},
		{"Pydantic timedelta(hours=-1)", `"-PT1H"`, -time.Hour},
		{"Pydantic timedelta(seconds=1.5)", `"PT1.5S"`, 1500 * time.Millisecond},
		{"Pydantic timedelta(microseconds=1)", `"PT0.000001S"`, time.Microsecond},
		{"Pydantic timedelta(days=1, microseconds=500)", `"P1DT0.0005S"`, day + 500*time.Microsecond},
		{"Pydantic timedelta(0)", `"PT0S"`, 0},
		{"XmlConvert TimeSpan.FromHours(36)", `"P1DT12H"`, 36 * time.Hour},
		{"XmlConvert TimeSpan.FromHours(-36)", `"-P1DT12H"`, -36 * time.Hour},
		{"XmlConvert TimeSpan.FromDays(14)", `"P14D"`, 2 * week},
		{"XmlConvert TimeSpan.FromTicks(1)", `"PT0.0000001S"`, 100 * time.Nanosecond},
		{"XmlConvert TimeSpan.FromMilliseconds(1500)", `"PT1.5S"`, 1500 * time.Millisecond},
		{"XmlConvert TimeSpan.Zero", `"PT0S"`, 0},
	}

	for _, tt := range tests {
		t.Run(tt.producer, func(t *testing.T) {
			var d time.Duration
			err := json.Unmarshal([]byte(tt.in), &d, opts)

			if err != nil || d != tt.want {
				t.Errorf("Unmarshal(%s) = %v, %v, want %v", tt.in, d, err, tt.want)
			}
		})
	}
}

// TestISO8601ReadsZeroCalendarUnits pins durations as producers write them that name calendar units with a count of
// zero. ISO8601 reads them without AcceptDays, since zero days are no time at all.
func TestISO8601ReadsZeroCalendarUnits(t *testing.T) {
	tests := []struct {
		producer string
		in       string
		want     time.Duration
	}{
		{"moment.duration(0).toISOString()", `"P0D"`, 0},
		{"isodate.duration_isoformat(timedelta(0))", `"P0D"`, 0},
		{"date-fns formatISODuration({})", `"P0Y0M0DT0H0M0S"`, 0},
		{"date-fns formatISODuration({hours: 1, minutes: 30})", `"P0Y0M0DT1H30M0S"`, 90 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.producer, func(t *testing.T) {
			var d time.Duration
			err := json.Unmarshal([]byte(tt.in), &d, With(ISO8601))

			if err != nil || d != tt.want {
				t.Errorf("Unmarshal(%s) = %v, %v, want %v", tt.in, d, err, tt.want)
			}
		})
	}
}

// TestISO8601DiffersFromJava lists every difference from Java's Duration.parse that a comparison on tricky input
// found, run with Java 26 on 2026-11-08, and why each one is intended. Both read the rest of that input alike.
func TestISO8601DiffersFromJava(t *testing.T) {
	opts := With(ISO8601, AcceptDays())

	tests := []struct {
		in      string
		want    time.Duration
		wantErr error
		java    string
		why     string
	}{
		{`"PT-1H"`, 0, strconv.ErrSyntax, "-1h", "a sign is allowed only in front"},
		{`"-PT-1H"`, 0, strconv.ErrSyntax, "1h", "a sign is allowed only in front"},
		{`"PT+1H"`, 0, strconv.ErrSyntax, "1h", "a sign is allowed only in front"},
		{`"P-1D"`, 0, strconv.ErrSyntax, "-24h", "a sign is allowed only in front"},
		{`"PT1.S"`, 0, strconv.ErrSyntax, "1s", "ISO 8601 wants a digit after the decimal sign"},
		{`"PT1.5H"`, 90 * time.Minute, nil, "error", "encoding/json/v2 read fractions of hours and minutes too"},
		{`"PT1.5M"`, 90 * time.Second, nil, "error", "encoding/json/v2 read fractions of hours and minutes too"},
		{`"P2W"`, 2 * week, nil, "error", "a week is 7 days of 24 hours, so AcceptDays reads it"},
		{`"PT0.0000000019S"`, 1, nil, "error", "digits below one nanosecond are truncated"},
		{`"P106752D"`, 0, strconv.ErrRange, "106752 days", "a time.Duration ends after about 292 years"},
		{`"PT9223372036.854775808S"`, 0, strconv.ErrRange, "2562047h47m16.854775808s", "one past the largest"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			var d time.Duration
			err := json.Unmarshal([]byte(tt.in), &d, opts)

			switch {
			case tt.wantErr != nil && !errors.Is(err, tt.wantErr):
				t.Errorf("Unmarshal(%s) error = %v, want %v (Java: %s; %s)", tt.in, err, tt.wantErr, tt.java, tt.why)
			case tt.wantErr == nil && (err != nil || d != tt.want):
				t.Errorf("Unmarshal(%s) = %v, %v, want %v (Java: %s; %s)", tt.in, d, err, tt.want, tt.java, tt.why)
			}
		})
	}
}
