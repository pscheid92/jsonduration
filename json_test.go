package jsonduration

import (
	"bytes"
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type object struct {
	D time.Duration
}

type payload struct {
	TTL  time.Duration                   `json:"ttl"`
	Ptr  *time.Duration                  `json:"ptr,omitempty"`
	List []time.Duration                 `json:"list"`
	Keys map[time.Duration]time.Duration `json:"keys"`
	Any  any                             `json:"any"`
}

func samplePayload() payload {
	d := d90
	return payload{
		TTL:  d90,
		Ptr:  &d,
		List: []time.Duration{time.Second, -time.Millisecond},
		Keys: map[time.Duration]time.Duration{time.Minute: time.Hour},
		Any:  time.Hour,
	}
}

func TestWith(t *testing.T) {
	wantJSON := map[Format]string{
		Units: `{"ttl":"1h30m0.5s","ptr":"1h30m0.5s","list":["1s","-1ms"],` +
			`"keys":{"1m0s":"1h0m0s"},"any":"1h0m0s"}`,
		Sec: `{"ttl":5400.5,"ptr":5400.5,"list":[1,-0.001],` +
			`"keys":{"60":3600},"any":3600}`,
		Milli: `{"ttl":5400500,"ptr":5400500,"list":[1000,-1],` +
			`"keys":{"60000":3600000},"any":3600000}`,
		Micro: `{"ttl":5400500000,"ptr":5400500000,"list":[1000000,-1000],` +
			`"keys":{"60000000":3600000000},"any":3600000000}`,
		Nano: `{"ttl":5400500000000,"ptr":5400500000000,"list":[1000000000,-1000000],` +
			`"keys":{"60000000000":3600000000000},"any":3600000000000}`,
		ISO8601: `{"ttl":"PT1H30M0.5S","ptr":"PT1H30M0.5S","list":["PT1S","-PT0.001S"],` +
			`"keys":{"PT1M":"PT1H"},"any":"PT1H"}`,
	}

	for _, f := range Formats() {
		t.Run(string(f), func(t *testing.T) {
			want, ok := wantJSON[f]
			if !ok {
				t.Fatalf("no expected JSON for format %s", f)
			}

			opts := With(f)
			p := samplePayload()

			b, err := json.Marshal(p, opts)
			if err != nil || string(b) != want {
				t.Fatalf("Marshal = %s, %v\nwant      %s", b, err, want)
			}

			var got payload
			if err := json.Unmarshal(b, &got, opts); err != nil {
				t.Fatalf("Unmarshal(%s): %v", b, err)
			}

			// A duration in an any comes back as a string or a number.
			got.Any, p.Any = nil, nil
			if !reflect.DeepEqual(got, p) {
				t.Errorf("Unmarshal(%s) = %+v, want %+v", b, got, p)
			}
		})
	}
}

func TestNanoMatchesV1(t *testing.T) {
	opts := With(Nano)
	p := samplePayload()

	v1, err := jsonv1.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}

	v2, err := json.Marshal(p, opts)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(v1, v2) {
		t.Errorf("Nano differs from encoding/json:\nv1 %s\nv2 %s", v1, v2)
	}

	var fromV1 payload
	if err := json.Unmarshal(v1, &fromV1, opts); err != nil || fromV1.TTL != d90 {
		t.Errorf("Unmarshal(v1 output) = %v, %v, want %v", fromV1.TTL, err, d90)
	}
}

func TestStringifyNumbers(t *testing.T) {
	opts := json.JoinOptions(With(Sec), json.StringifyNumbers(true))
	v := object{D: d90}
	const want = `{"D":"5400.5"}`

	b, err := json.Marshal(v, opts)
	if err != nil || string(b) != want {
		t.Errorf("Marshal = %s, %v, want %s", b, err, want)
	}

	var got object
	if err := json.Unmarshal(b, &got, opts); err != nil || got.D != d90 {
		t.Errorf("Unmarshal(%s) = %v, %v, want %v", b, got.D, err, d90)
	}
}

func TestUnmarshalIsStrict(t *testing.T) {
	tests := []struct {
		name    string
		f       Format
		in      string
		wantErr string
	}{
		{"string for a numeric format", Sec, `{"D":"5400.5"}`, "want a JSON number"},
		{"number for a string format", ISO8601, `{"D":5400}`, "want a JSON string"},
		{"units for ISO 8601", ISO8601, `{"D":"1h30m"}`, "in format iso8601"},
		{"ISO 8601 for units", Units, `{"D":"PT1H"}`, "invalid duration"},
		{"calendar units", ISO8601, `{"D":"P1D"}`, "no fixed length"},
		{"boolean", Nano, `{"D":true}`, "want a JSON number"},
		{"exponent", Nano, `{"D":1e3}`, "invalid duration"},
		{"number without a unit", Units, `{"D":"1.5"}`, "missing unit"},
		{"overflow", Sec, `{"D":99999999999}`, "value out of range"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := With(tt.f)

			var v object
			err := json.Unmarshal([]byte(tt.in), &v, opts)

			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Unmarshal(%s) error = %v, want one containing %q", tt.in, err, tt.wantErr)
			}

			if !strings.Contains(err.Error(), `"/D"`) {
				t.Errorf("Unmarshal(%s) error %q does not name the JSON pointer", tt.in, err)
			}
		})
	}
}

func TestNumericMapKeys(t *testing.T) {
	opts := json.JoinOptions(With(Sec), json.Deterministic(true))
	m := map[time.Duration]time.Duration{time.Second: time.Minute, time.Minute: time.Hour, time.Hour: time.Second}
	const want = `{"1":60,"3600":1,"60":3600}`

	b, err := json.Marshal(m, opts)
	if err != nil || string(b) != want {
		t.Fatalf("Marshal = %s, %v, want %s", b, err, want)
	}

	var got map[time.Duration]time.Duration
	if err := json.Unmarshal(b, &got, opts); err != nil || len(got) != len(m) || got[time.Hour] != time.Second {
		t.Errorf("Unmarshal(%s) = %v, %v, want %v", b, got, err, m)
	}
}

func TestUnmarshalNull(t *testing.T) {
	opts := With(ISO8601)
	v := object{D: time.Hour}

	err := json.Unmarshal([]byte(`{"D":null}`), &v, opts)
	if err != nil || v.D != 0 {
		t.Errorf("Unmarshal(null) = %v, %v, want 0", v.D, err)
	}
}

func TestUnmarshalEscapedString(t *testing.T) {
	opts := With(ISO8601)

	// An escape at the start, in the middle, and at the end, each spelling "PT1H".
	inputs := []string{`"\u0050T1H"`, `"P\u0054\u0031H"`, `"PT1\u0048"`}

	for _, in := range inputs {
		var d time.Duration
		err := json.Unmarshal([]byte(in), &d, opts)
		if err != nil || d != time.Hour {
			t.Errorf("Unmarshal(%s) = %v, %v, want 1h", in, d, err)
		}
	}
}

func TestV2StillHasNoDurationDefault(t *testing.T) {
	p := samplePayload()

	if _, err := json.Marshal(p); err == nil {
		t.Fatal("encoding/json/v2 now has a default for time.Duration; revisit this package")
	}
}

func TestStringTagQuotesNumericFormats(t *testing.T) {
	type config struct {
		Timeout time.Duration `json:",string"`
		Backoff time.Duration
	}

	opts := With(Sec)
	v := config{Timeout: d90, Backoff: d90}
	const want = `{"Timeout":"5400.5","Backoff":5400.5}`

	b, err := json.Marshal(v, opts)
	if err != nil || string(b) != want {
		t.Errorf("Marshal = %s, %v, want %s", b, err, want)
	}

	var got config
	if err := json.Unmarshal([]byte(want), &got, opts); err != nil || got != v {
		t.Errorf("Unmarshal(%s) = %+v, %v, want %+v", want, got, err, v)
	}

	// As for an int with the option, a JSON number is rejected.
	unquoted := []byte(`{"Timeout":5400.5}`)
	if err := json.Unmarshal(unquoted, &got, opts); err == nil {
		t.Errorf("Unmarshal(%s) succeeded, want an error", unquoted)
	}

	isoOpts := With(ISO8601)
	const wantISO = `{"Timeout":"PT1H30M0.5S","Backoff":"PT1H30M0.5S"}`

	b, err = json.Marshal(v, isoOpts)
	if err != nil || string(b) != wantISO {
		t.Errorf("Marshal = %s, %v, want %s", b, err, wantISO)
	}
}

func TestNamedTypesAreNotAffected(t *testing.T) {
	type TTL time.Duration

	opts := With(ISO8601)
	ttl := TTL(d90)
	const want = `5400500000000`

	b, err := json.Marshal(ttl, opts)
	if err != nil || string(b) != want {
		t.Errorf("Marshal(TTL) = %s, %v, want %s, as for any int64", b, err, want)
	}
}

func TestJoinWithOwnMarshalers(t *testing.T) {
	type ID string

	type record struct {
		ID ID
		D  time.Duration
	}

	writeID := func(enc *jsontext.Encoder, id ID) error {
		token := jsontext.String("id-" + string(id))
		return enc.WriteToken(token)
	}

	ids := json.MarshalToFunc(writeID)
	durations := Marshalers(Units)
	opts := json.WithMarshalers(json.JoinMarshalers(ids, durations))

	v := record{ID: "x", D: time.Second}
	const want = `{"ID":"id-x","D":"1s"}`

	b, err := json.Marshal(v, opts)
	if err != nil || string(b) != want {
		t.Errorf("Marshal = %s, %v, want %s", b, err, want)
	}
}

func TestEncoderStreaming(t *testing.T) {
	opts := With(ISO8601)
	durations := []time.Duration{time.Second, 1500 * time.Millisecond}
	const want = "\"PT1S\"\n\"PT1.5S\"\n"

	var sb strings.Builder
	enc := jsontext.NewEncoder(&sb, opts)

	for _, d := range durations {
		if err := json.MarshalEncode(enc, d); err != nil {
			t.Fatal(err)
		}
	}

	if got := sb.String(); got != want {
		t.Errorf("stream = %q, want %q", got, want)
	}
}

// TestConcurrentUse only finds a data race when run with -race, as CI does.
func TestConcurrentUse(t *testing.T) {
	p := samplePayload()

	for _, f := range Formats() {
		opts := With(f)

		want, err := json.Marshal(p, opts)
		if err != nil {
			t.Fatal(err)
		}

		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				for range 50 {
					b, err := json.Marshal(p, opts)
					if err != nil || !bytes.Equal(b, want) {
						t.Errorf("%s: Marshal = %s, %v, want %s", f, b, err, want)
						return
					}

					var got payload
					if err := json.Unmarshal(b, &got, opts); err != nil || got.TTL != d90 {
						t.Errorf("%s: Unmarshal = %v, %v, want %v", f, got.TTL, err, d90)
						return
					}
				}
			})
		}

		wg.Wait()
	}
}

func TestAlsoAccept(t *testing.T) {
	type doc struct {
		D time.Duration                   `json:"d"`
		M map[time.Duration]time.Duration `json:"m"`
	}

	opts := With(ISO8601, AlsoAccept(Nano))

	reads := map[string]string{
		"v1 nanoseconds":       `{"d":5400500000000,"m":{"60000000000":3600000000000}}`,
		"ISO 8601":             `{"d":"PT1H30M0.5S","m":{"PT1M":"PT1H"}}`,
		"both in one document": `{"d":"PT1H30M0.5S","m":{"60000000000":"PT1H"}}`,
	}

	for name, in := range reads {
		t.Run("reads "+name, func(t *testing.T) {
			var v doc
			err := json.Unmarshal([]byte(in), &v, opts)
			if err != nil || v.D != d90 || v.M[time.Minute] != time.Hour {
				t.Errorf("Unmarshal(%s) = %+v, %v", in, v, err)
			}
		})
	}

	t.Run("writes only the first format", func(t *testing.T) {
		v := doc{
			D: d90,
			M: map[time.Duration]time.Duration{time.Minute: time.Hour},
		}

		const want = `{"d":"PT1H30M0.5S","m":{"PT1M":"PT1H"}}`

		b, err := json.Marshal(v, opts)
		if err != nil || string(b) != want {
			t.Errorf("Marshal = %s, %v, want %s", b, err, want)
		}
	})

	t.Run("reads a numeric format as a string with StringifyNumbers", func(t *testing.T) {
		unitsOrSec := With(Units, AlsoAccept(Sec))
		stringify := json.StringifyNumbers(true)

		var d time.Duration
		err := json.Unmarshal([]byte(`"5400.5"`), &d, unitsOrSec, stringify)
		if err != nil || d != d90 {
			t.Errorf("Unmarshal = %v, %v, want %v", d, err, d90)
		}
	})

	t.Run("keeps its formats when the caller changes the slice", func(t *testing.T) {
		formats := []Format{Units}
		accept := AlsoAccept(formats...)
		formats[0] = Nano

		secOrUnits := With(Sec, accept)

		var d time.Duration
		err := json.Unmarshal([]byte(`"1h"`), &d, secOrUnits)
		if err != nil || d != time.Hour {
			t.Errorf("Unmarshal = %v, %v, want 1h", d, err)
		}
	})

	t.Run("ignores duplicates and nil options", func(t *testing.T) {
		withDuplicates := With(ISO8601, AlsoAccept(ISO8601), nil)

		var d time.Duration
		err := json.Unmarshal([]byte(`"x"`), &d, withDuplicates)
		if err == nil || strings.Contains(err.Error(), "accepted") {
			t.Errorf("Unmarshal error = %v, want the error of ISO 8601 alone", err)
		}
	})

	t.Run("keeps the formats after a duplicate", func(t *testing.T) {
		withDuplicate := With(ISO8601, AlsoAccept(ISO8601, Units))

		var d time.Duration
		err := json.Unmarshal([]byte(`"1h"`), &d, withDuplicate)
		if err != nil || d != time.Hour {
			t.Errorf("Unmarshal = %v, %v, want 1h", d, err)
		}
	})
}

func TestAlsoAcceptErrors(t *testing.T) {
	opts := With(ISO8601, AlsoAccept(Nano))

	tests := []struct {
		name    string
		in      string
		wantErr error
		wantMsg string
	}{
		{"units", `{"D":"1h30m"}`, strconv.ErrSyntax, `"1h30m" in format iso8601 or nano: invalid syntax`},
		{"calendar units", `{"D":"P1D"}`, ErrCalendarUnits, "iso8601 or nano: calendar units have no fixed length"},
		{"exponent", `{"D":1e3}`, strconv.ErrSyntax, `invalid duration "1e3"`},
		{"nanoseconds as a string value", `{"D":"5400"}`, strconv.ErrSyntax, "in format iso8601"},
		{"boolean", `{"D":true}`, nil, `"true" in format iso8601 or nano: want a JSON string or number`},
		{"array", `{"D":[]}`, nil, "want a JSON string or number"},
		{"object", `{"D":{}}`, nil, "want a JSON string or number"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v object
			err := json.Unmarshal([]byte(tt.in), &v, opts)

			if err == nil || !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("Unmarshal(%s) error = %v, want one containing %q", tt.in, err, tt.wantMsg)
			}

			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("Unmarshal(%s) error = %v, want one wrapping %v", tt.in, err, tt.wantErr)
			}
		})
	}
}

// TestAlsoAcceptReportsTheTellingCause checks that a format that recognized the duration but not its value
// decides the cause, whichever comes first, as for a number too large for nano.
func TestAlsoAcceptReportsTheTellingCause(t *testing.T) {
	type config struct {
		D time.Duration
		M map[time.Duration]int
	}

	unitsOrISO := With(Units, AlsoAccept(ISO8601))
	isoOrUnits := With(ISO8601, AlsoAccept(Units))
	isoOrNano := With(ISO8601, AlsoAccept(Nano))
	stringified := json.JoinOptions(isoOrNano, json.StringifyNumbers(true))

	tests := []struct {
		name    string
		in      string
		opts    json.Options
		wantErr error
		wantMsg string
	}{
		{"calendar units second", `{"D":"P1D"}`, unitsOrISO, ErrCalendarUnits, "no fixed length"},
		{"calendar units first", `{"D":"P1D"}`, isoOrUnits, ErrCalendarUnits, "no fixed length"},
		{"range in a map key", `{"M":{"99999999999999999999":1}}`, isoOrNano, strconv.ErrRange, "range"},
		{"range with StringifyNumbers", `{"D":"99999999999999999999"}`, stringified, strconv.ErrRange, "range"},
		{"first syntax error", `{"D":"1x"}`, isoOrUnits, strconv.ErrSyntax, "iso8601 or units: invalid syntax"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v config
			err := json.Unmarshal([]byte(tt.in), &v, tt.opts)
			if !errors.Is(err, tt.wantErr) || !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("Unmarshal(%s) error = %v, want one wrapping %v and containing %q",
					tt.in, err, tt.wantErr, tt.wantMsg)
			}
		})
	}
}

// TestDecodeErrorsAreSemanticErrors checks that decoding reports errors as encoding/json/v2 does for other types,
// with the JSON kind, the Go type, and the JSON pointer, so that code that handles its errors also handles durations.
func TestDecodeErrorsAreSemanticErrors(t *testing.T) {
	type config struct {
		Timeout time.Duration
		Backoff ISO8601Duration
		Limits  map[time.Duration]int
	}

	strict := With(ISO8601)
	both := With(ISO8601, AlsoAccept(Nano))
	duration := reflect.TypeFor[time.Duration]()
	wrapper := reflect.TypeFor[ISO8601Duration]()

	tests := []struct {
		name        string
		in          string
		opts        json.Options
		wantKind    jsontext.Kind
		wantType    reflect.Type
		wantPointer jsontext.Pointer
	}{
		{"invalid syntax", `{"Timeout":"90s"}`, strict, '"', duration, "/Timeout"},
		{"calendar units", `{"Timeout":"P1D"}`, strict, '"', duration, "/Timeout"},
		{"out of range", `{"Timeout":99999999999999999999}`, both, '0', duration, "/Timeout"},
		{"number for ISO8601", `{"Timeout":90}`, strict, '0', duration, "/Timeout"},
		{"boolean for AlsoAccept", `{"Timeout":true}`, both, 't', duration, "/Timeout"},
		{"map key", `{"Limits":{"90s":1}}`, strict, '"', duration, "/Limits/90s"},
		{"invalid syntax in a wrapper type", `{"Backoff":"90s"}`, strict, '"', wrapper, "/Backoff"},
		{"array for a wrapper type", `{"Backoff":[]}`, strict, '[', wrapper, "/Backoff"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v config
			err := json.Unmarshal([]byte(tt.in), &v, tt.opts)

			semantic, ok := errors.AsType[*json.SemanticError](err)
			if !ok {
				t.Fatalf("error %v, want a json.SemanticError", err)
			}

			if semantic.JSONKind != tt.wantKind || semantic.GoType != tt.wantType {
				t.Errorf("error has JSONKind %v and GoType %v, want %v and %v",
					semantic.JSONKind, semantic.GoType, tt.wantKind, tt.wantType)
			}

			if semantic.JSONPointer != tt.wantPointer {
				t.Errorf("error has JSONPointer %q, want %q", semantic.JSONPointer, tt.wantPointer)
			}

			if !errors.Is(err, ErrInvalidDuration) {
				t.Errorf("error %v does not match ErrInvalidDuration", err)
			}
		})
	}
}

func TestAcceptDays(t *testing.T) {
	opts := With(ISO8601, AcceptDays())

	tests := []struct {
		in      string
		want    time.Duration
		wantErr error
	}{
		{`"P1D"`, day, nil},
		{`"p2d"`, 2 * day, nil},
		{`"P1W"`, week, nil},
		{`"P1W2D"`, week + 2*day, nil},
		{`"P1DT12H"`, 36 * time.Hour, nil},
		{`"-P1D"`, -day, nil},
		{`"P106751DT23H47M16.854775807S"`, maxD, nil},
		{`"P106752D"`, 0, strconv.ErrRange},
		{`"P1Y"`, 0, ErrCalendarUnits},
		{`"P1M"`, 0, ErrCalendarUnits},
		{`"P1M1D"`, 0, ErrCalendarUnits},
		{`"P0D"`, 0, nil},
		{`"P0Y0M1DT1H"`, 25 * time.Hour, nil},
		{`"P0Y1M0D"`, 0, ErrCalendarUnits},
		{`"P0.0D"`, 0, strconv.ErrSyntax},
		{`"P1.5D"`, 0, strconv.ErrSyntax},
		{`"P1DT"`, 0, strconv.ErrSyntax},
		{`"P1D1"`, 0, strconv.ErrSyntax},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			var d time.Duration
			err := json.Unmarshal([]byte(tt.in), &d, opts)

			switch {
			case tt.wantErr != nil && !errors.Is(err, tt.wantErr):
				t.Errorf("Unmarshal(%s) error = %v, want %v", tt.in, err, tt.wantErr)
			case tt.wantErr == nil && (err != nil || d != tt.want):
				t.Errorf("Unmarshal(%s) = %v, %v, want %v", tt.in, d, err, tt.want)
			}
		})
	}

	t.Run("days stay rejected without it", func(t *testing.T) {
		withoutDays := With(ISO8601)

		var d time.Duration
		err := json.Unmarshal([]byte(`"P1D"`), &d, withoutDays)
		if !errors.Is(err, ErrCalendarUnits) {
			t.Errorf("Unmarshal error = %v, want ErrCalendarUnits", err)
		}
	})

	t.Run("never writes days", func(t *testing.T) {
		d := 36 * time.Hour
		const want = `"PT36H"`

		b, err := json.Marshal(d, opts)
		if err != nil || string(b) != want {
			t.Errorf("Marshal = %s, %v, want %s", b, err, want)
		}
	})

	t.Run("works with ISO 8601 from AlsoAccept", func(t *testing.T) {
		nanoOrDays := With(Nano, AlsoAccept(ISO8601), AcceptDays())

		var d time.Duration
		err := json.Unmarshal([]byte(`"P1D"`), &d, nanoOrDays)
		if err != nil || d != day {
			t.Errorf("Unmarshal = %v, %v, want %v", d, err, day)
		}
	})

	t.Run("leaves other formats alone", func(t *testing.T) {
		unitsWithDays := With(Units, AcceptDays())

		var d time.Duration
		err := json.Unmarshal([]byte(`"1h"`), &d, unitsWithDays)
		if err != nil || d != time.Hour {
			t.Errorf("Unmarshal = %v, %v, want 1h", d, err)
		}
	})
}

func TestUnmarshalSyntaxError(t *testing.T) {
	opts := With(Nano)

	var v object
	err := json.Unmarshal([]byte(`{"D":tru}`), &v, opts)

	if _, ok := errors.AsType[*jsontext.SyntacticError](err); !ok {
		t.Errorf("Unmarshal error = %v, want a *jsontext.SyntacticError", err)
	}
}

func TestUnmarshalInvalidUTF8(t *testing.T) {
	opts := json.JoinOptions(With(ISO8601), jsontext.AllowInvalidUTF8(true))

	tests := map[string]string{
		"without escapes": "\"PT1H\xff\"",
		"with escapes":    "\"PT1\\u0048\xff\"",
		"lone surrogate":  "\"\\ud800PT1H\"",
	}

	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			var d time.Duration
			err := json.Unmarshal([]byte(in), &d, opts)
			if !errors.Is(err, ErrInvalidDuration) || !errors.Is(err, strconv.ErrSyntax) {
				t.Errorf("Unmarshal(%q) = %v, %v, want an error that matches ErrInvalidDuration and strconv.ErrSyntax",
					in, d, err)
			}
		})
	}
}

// FuzzUnmarshal decodes arbitrary JSON with every format, and with the options that combine formats.
// Whatever decodes must survive a second round trip unchanged.
func FuzzUnmarshal(f *testing.F) {
	type doc struct {
		D time.Duration                   `json:"d"`
		P *time.Duration                  `json:"p"`
		L []time.Duration                 `json:"l"`
		M map[time.Duration]time.Duration `json:"m"`
	}

	type config struct {
		name string
		opts json.Options
	}

	seeds := []string{
		`{"d":"PT1H30M0.5S","p":null,"l":["1s","-1ms"],"m":{"60":3600}}`,
		`{"d":5400.5,"l":[1,-0.001],"m":{"PT1M":"PT1H"}}`,
		`{"d":"1h30m","l":[null]}`,
		`{"d":"P1DT2H","p":"PT1S"}`,
		`[1,2,3]`, `null`, `{"d":"PT1H"}`,
	}

	for _, s := range seeds {
		f.Add([]byte(s))
	}

	var configs []config
	add := func(name string, opts json.Options) {
		deterministic := json.JoinOptions(opts, json.Deterministic(true))
		configs = append(configs, config{name: name, opts: deterministic})
	}

	for _, fm := range Formats() {
		add(string(fm), With(fm))
		if !fm.mustCodec().numeric() {
			add(string(fm)+"+nano", With(fm, AlsoAccept(Nano)))
		}
	}

	add("iso8601+days", With(ISO8601, AcceptDays()))

	f.Fuzz(func(t *testing.T, in []byte) {
		for _, c := range configs {
			var v doc
			if json.Unmarshal(in, &v, c.opts) != nil {
				continue
			}

			b1, err := json.Marshal(v, c.opts)
			if err != nil {
				t.Fatalf("%s: Marshal after Unmarshal(%q): %v", c.name, in, err)
			}

			var w doc
			if err := json.Unmarshal(b1, &w, c.opts); err != nil {
				t.Fatalf("%s: Unmarshal(%s): %v", c.name, b1, err)
			}

			if b2, _ := json.Marshal(w, c.opts); !bytes.Equal(b1, b2) {
				t.Fatalf("%s: round trip changed\n%s\n%s", c.name, b1, b2)
			}
		}
	})
}
