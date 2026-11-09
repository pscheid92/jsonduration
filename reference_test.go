package jsonduration

import (
	"encoding/json/jsontext"
	"errors"
	"math"
	"math/big"
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The reference parsers in this file are deliberately different from the real ones: regular expressions for the
// syntax and math/big for exact arithmetic, without sharing any code with the package. FuzzParseMatchesReference and
// TestParseMatchesReference check that both agree on whether an input is accepted, on its value, and on the cause of
// an error, which is strconv.ErrSyntax, strconv.ErrRange, or ErrCalendarUnits.

// referenceDuration returns the Duration with absolute value abs and the given sign, or strconv.ErrRange.
func referenceDuration(abs *big.Int, negative bool) (time.Duration, error) {
	limit := big.NewInt(math.MaxInt64)
	if negative {
		limit = new(big.Int).Add(limit, big.NewInt(1))
	}

	if abs.Cmp(limit) > 0 {
		return 0, strconv.ErrRange
	}

	value := new(big.Int).Set(abs)
	if negative {
		value.Neg(value)
	}

	return time.Duration(value.Int64()), nil
}

// referenceScale returns whole.fraction × unit, rounded down to whole nanoseconds.
func referenceScale(whole, fraction string, unit int64) *big.Int {
	digits, _ := new(big.Int).SetString("0"+whole+fraction, 10)
	digits.Mul(digits, big.NewInt(unit))

	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(len(fraction))), nil)
	return digits.Quo(digits, divisor)
}

var referenceDecimalSyntax = regexp.MustCompile(`^(-?)(0|[1-9]\d*)(?:\.(\d+))?$`)

func referenceDecimal(s string, unit int64) (time.Duration, error) {
	m := referenceDecimalSyntax.FindStringSubmatch(s)
	if m == nil {
		return 0, strconv.ErrSyntax
	}

	abs := referenceScale(m[2], m[3], unit)
	return referenceDuration(abs, m[1] == "-")
}

var (
	referenceUnitsSyntax = regexp.MustCompile(`^([-+]?)(?:0|` +
		`(?:(?:\d+\.?\d*|\.\d+)(?:ns|us|µs|μs|ms|s|m|h))+)$`)
	referenceUnitsPart   = regexp.MustCompile(`(\d*)(?:\.(\d*))?(ns|us|µs|μs|ms|s|m|h)`)
	referenceUnitLengths = map[string]int64{
		"ns": 1, "us": 1e3, "µs": 1e3, "μs": 1e3, "ms": 1e6, "s": 1e9, "m": 60e9, "h": 3600e9,
	}
)

func referenceUnits(s string) (time.Duration, error) {
	m := referenceUnitsSyntax.FindStringSubmatch(s)
	if m == nil {
		return 0, strconv.ErrSyntax
	}

	body := strings.TrimLeft(s, "+-")
	if body == "0" {
		return 0, nil
	}

	abs := new(big.Int)
	for _, part := range referenceUnitsPart.FindAllStringSubmatch(body, -1) {
		abs.Add(abs, referenceScale(part[1], part[2], referenceUnitLengths[part[3]]))
	}

	return referenceDuration(abs, m[1] == "-")
}

// referenceISO8601Syntax matches the shape of the ISO 8601 profile: every component may have a fraction here, and
// referenceISO8601 then checks which ones may.
var referenceISO8601Syntax = regexp.MustCompile(`(?i)^([-+]?)P` +
	`(?:(\d+)(?:[.,](\d+))?Y)?(?:(\d+)(?:[.,](\d+))?M)?` +
	`(?:(\d+)(?:[.,](\d+))?W)?(?:(\d+)(?:[.,](\d+))?D)?` +
	`(T(?:(\d+)(?:[.,](\d+))?H)?(?:(\d+)(?:[.,](\d+))?M)?(?:(\d+)(?:[.,](\d+))?S)?)?$`)

func referenceISO8601(s string, acceptDays bool) (time.Duration, error) {
	m := referenceISO8601Syntax.FindStringSubmatchIndex(s)
	if m == nil {
		return 0, strconv.ErrSyntax
	}

	group := func(i int) (string, bool) {
		if m[2*i] < 0 {
			return "", false
		}
		return s[m[2*i]:m[2*i+1]], true
	}

	type component struct {
		whole, fraction string
		present         bool
		hasFraction     bool
		unit            int64
		calendar        bool
	}

	const day = 24 * 3600e9
	components := make([]component, 0, 7)
	// Years and months are always calendar units, so their units are unused.
	for i, unit := range []int64{0, 0, 7 * day, day, 3600e9, 60e9, 1e9} {
		whole, present := group(2 + 2*i)
		if i >= 4 {
			whole, present = group(3 + 2*i)
		}

		fractionGroup := 3 + 2*i
		if i >= 4 {
			fractionGroup = 4 + 2*i
		}
		fraction, hasFraction := group(fractionGroup)

		// A count of zero is no time at all, whatever the length of the unit.
		isZero := present && strings.Trim(whole, "0") == "" && !hasFraction
		calendar := (i <= 1 || i <= 3 && !acceptDays) && !isZero
		components = append(components, component{whole, fraction, present, hasFraction, unit, calendar})
	}

	_, hasT := group(10)
	last, anyClock := -1, false
	for i, c := range components {
		if c.present {
			last = i
			anyClock = anyClock || i >= 4
		}
	}

	switch {
	case last < 0, hasT && !anyClock:
		return 0, strconv.ErrSyntax
	}

	calendar := false
	for i, c := range components {
		if !c.present {
			continue
		}

		if c.hasFraction && i != last {
			return 0, strconv.ErrSyntax
		}

		// Days and weeks read with acceptDays take no fraction.
		if c.hasFraction && !c.calendar && i <= 3 {
			return 0, strconv.ErrSyntax
		}

		calendar = calendar || c.calendar
	}

	if calendar {
		return 0, ErrCalendarUnits
	}

	abs := new(big.Int)
	for _, c := range components {
		if c.present {
			abs.Add(abs, referenceScale(c.whole, c.fraction, c.unit))
		}
	}

	negative, _ := group(1)
	return referenceDuration(abs, negative == "-")
}

func reference(f Format, s string) (time.Duration, error) {
	switch f {
	case Units:
		return referenceUnits(s)
	case Sec:
		return referenceDecimal(s, 1e9)
	case Milli:
		return referenceDecimal(s, 1e6)
	case Micro:
		return referenceDecimal(s, 1e3)
	case Nano:
		return referenceDecimal(s, 1)
	default:
		return referenceISO8601(s, false)
	}
}

func checkAgainstReference(
	t *testing.T, name, s string, got time.Duration, err error, want time.Duration, wantErr error,
) {
	t.Helper()

	switch {
	case wantErr == nil && (err != nil || got != want):
		t.Fatalf("%s(%q) = %d, %v, want %d", name, s, int64(got), err, int64(want))
	case wantErr != nil && (!errors.Is(err, wantErr) || got != 0):
		t.Fatalf("%s(%q) = %d, %v, want 0 and an error caused by %v", name, s, int64(got), err, wantErr)
	}
}

// A referenceResult is what the reference reads from one input in one format.
type referenceResult struct {
	d   time.Duration
	err error
}

// checkAllFormats checks every format against the reference, on its own and in pairs, as AlsoAccept combines them.
func checkAllFormats(t *testing.T, s string) {
	t.Helper()

	results := make(map[Format]referenceResult, len(Formats()))
	for _, f := range Formats() {
		want, wantErr := reference(f, s)
		results[f] = referenceResult{want, wantErr}

		got, err := f.Parse(s)
		checkAgainstReference(t, string(f)+".Parse", s, got, err, want, wantErr)

		if err != nil && !errors.Is(err, ErrInvalidDuration) {
			t.Fatalf("%s.Parse(%q) error %v does not match ErrInvalidDuration", f, s, err)
		}

		// The codec on its own, which JSON decoding uses, returns the bare cause.
		got, cause := f.mustCodec().parse([]byte(s))
		checkAgainstReference(t, string(f)+" codec", s, got, cause, want, wantErr)
	}

	want, wantErr := referenceISO8601(s, true)
	days := referenceResult{want, wantErr}

	got, err := codecISO8601Days.parse([]byte(s))
	checkAgainstReference(t, "iso8601 with AcceptDays", s, got, err, want, wantErr)

	checkPairs(t, s, results, days)
}

// A formatPair is a decoder for two formats, as With(first, AlsoAccept(second)) creates it.
type formatPair struct {
	name          string
	first, second Format
	acceptDays    bool
	dc            *decoder
}

// formatPairs holds every pair of formats that a decoder accepts, at most one of them numeric,
// and the pairs with ISO 8601 also with AcceptDays.
var formatPairs = newFormatPairs()

func newFormatPairs() []formatPair {
	var pairs []formatPair
	for _, first := range Formats() {
		for _, second := range Formats() {
			bothNumeric := first.mustCodec().numeric() && second.mustCodec().numeric()
			if first == second || bothNumeric {
				continue
			}

			hasISO8601 := first == ISO8601 || second == ISO8601
			for _, acceptDays := range []bool{false, true} {
				if acceptDays && !hasISO8601 {
					continue
				}

				name := string(first) + " or " + string(second)
				opts := []DecodeOption{AlsoAccept(second)}
				if acceptDays {
					name += " with AcceptDays"
					opts = append(opts, AcceptDays())
				}

				p := formatPair{name, first, second, acceptDays, newDecoder(first, opts)}
				pairs = append(pairs, p)
			}
		}
	}

	return pairs
}

// checkPairs checks every pair of formats against the reference results for s.
// The decoders read s from a JSON string, as in a map key, so that both formats try it.
func checkPairs(t *testing.T, s string, results map[Format]referenceResult, days referenceResult) {
	t.Helper()

	val, err := jsontext.AppendQuote(nil, s)
	if err != nil {
		return // invalid UTF-8, which AppendQuote replaces
	}

	resultFor := func(f Format, acceptDays bool) referenceResult {
		if f == ISO8601 && acceptDays {
			return days
		}

		return results[f]
	}

	for _, p := range formatPairs {
		first := resultFor(p.first, p.acceptDays)
		second := resultFor(p.second, p.acceptDays)
		want, wantErr := pairResult(t, p, s, first, second)

		got, err := p.dc.parse(val, true)
		checkAgainstReference(t, p.name, s, got, err, want, wantErr)
	}
}

// pairResult returns what a decoder for two formats reads, given what each format reads on its own:
// the duration of the first format that reads s, or else the cause of the format that recognized s,
// such as strconv.ErrRange, or else the syntax error of the first.
func pairResult(t *testing.T, p formatPair, s string, first, second referenceResult) (time.Duration, error) {
	t.Helper()

	if first.err == nil {
		return first.d, nil
	}

	if second.err == nil {
		return second.d, nil
	}

	firstRecognized := !errors.Is(first.err, strconv.ErrSyntax)
	secondRecognized := !errors.Is(second.err, strconv.ErrSyntax)
	if firstRecognized && secondRecognized {
		t.Fatalf("%s: both formats recognize %q, so the cause to report is not defined", p.name, s)
	}

	if secondRecognized {
		return 0, second.err
	}

	return 0, first.err
}

func FuzzParseMatchesReference(f *testing.F) {
	seeds := []string{
		"0", "-0", "+0", "1h30m0.5s", ".5s", "5.s", "1.000000000000000000001ms", "9223372036854775807ns",
		"-9223372036854775808ns", "9223372036854775808ns", "2562047h47m16.854775807s", "1µs", "1μs", "1h1h",
		"5400.5", "-0.000000001", "9223372036.854775807", "-9223372036.854775808", "01", "1.", "1e3",
		"PT1H30M0.5S", "pt1h30m", "+PT1,5S", "-PT2562047H47M16.854775808S", "P1D", "P1W2DT1.5H", "P0.5D", "P1.5Y",
		"P0D", "P0Y0M0DT1H30M0S", "P0Y1D", "P0.0D", "P00W",
		"PT1.5H30M", "P1DT", "PT", "P", "P106751DT23H47M16.854775807S", "P106752D", "PT0.0000000000002H",
		"", "-", "+", "PT1.5.5S", "99999999999999999999", "P99999999999999999999D",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		checkAllFormats(t, s)
	})
}

// TestParseMatchesReference feeds both parsers well-formed inputs with random digits, which fuzzing alone reaches
// only slowly: long fractions, values near the limits, signs, and every combination of ISO 8601 components.
func TestParseMatchesReference(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))

	digits := func(minLen, maxLen int) string {
		n := minLen + r.IntN(maxLen-minLen+1)
		b := make([]byte, n)
		for i := range b {
			b[i] = byte('0' + r.IntN(10))
		}

		return string(b)
	}

	number := func() string {
		whole := digits(1, 20)
		if r.IntN(3) == 0 {
			return whole + "." + digits(1, 30)
		}

		return whole
	}

	sign := func() string {
		return []string{"", "-", "+"}[r.IntN(3)]
	}

	units := []string{"ns", "us", "µs", "μs", "ms", "s", "m", "h"}
	designators := []string{"Y", "M", "W", "D", "H", "M", "S"}

	for range 50000 {
		var b strings.Builder

		switch r.IntN(3) {
		case 0:
			b.WriteString(sign())
			for range 1 + r.IntN(4) {
				b.WriteString(number())
				b.WriteString(units[r.IntN(len(units))])
			}
		case 1:
			b.WriteString(sign())
			b.WriteString(number())
		default:
			b.WriteString(sign())
			b.WriteString("P")
			for i, d := range designators {
				if i == 4 {
					b.WriteString("T")
				}

				if r.IntN(3) == 0 {
					b.WriteString(number())
					b.WriteString(d)
				}
			}
		}

		checkAllFormats(t, b.String())
	}
}
