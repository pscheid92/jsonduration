// Copyright 2026 Patrick Scheid. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonduration

import (
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

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

func TestMarshal(t *testing.T) {
	tests := map[Format]string{
		Units:     `{"ttl":"1h30m0.5s","ptr":"1h30m0.5s","list":["1s","-1ms"],"keys":{"1m0s":"1h0m0s"},"any":"1h0m0s"}`,
		Sec:       `{"ttl":5400.5,"ptr":5400.5,"list":[1,-0.001],"keys":{"60":3600},"any":3600}`,
		Milli:     `{"ttl":5400500,"ptr":5400500,"list":[1000,-1],"keys":{"60000":3600000},"any":3600000}`,
		Micro:     `{"ttl":5400500000,"ptr":5400500000,"list":[1000000,-1000],"keys":{"60000000":3600000000},"any":3600000000}`,
		Nano:      `{"ttl":5400500000000,"ptr":5400500000000,"list":[1000000000,-1000000],"keys":{"60000000000":3600000000000},"any":3600000000000}`,
		ISO8601:   `{"ttl":"PT1H30M0.5S","ptr":"PT1H30M0.5S","list":["PT1S","-PT0.001S"],"keys":{"PT1M":"PT1H"},"any":"PT1H"}`,
		Base60:    `{"ttl":"1:30:00.5","ptr":"1:30:00.5","list":["0:00:01","-0:00:00.001"],"keys":{"0:01:00":"1:00:00"},"any":"1:00:00"}`,
		ProtoJSON: `{"ttl":"5400.500s","ptr":"5400.500s","list":["1s","-0.001s"],"keys":{"60s":"3600s"},"any":"3600s"}`,
	}
	for _, f := range Formats() {
		want, ok := tests[f]
		if !ok {
			t.Errorf("no test for format %s", f)
			continue
		}
		b, err := json.Marshal(samplePayload(), With(f))
		if err != nil || string(b) != want {
			t.Errorf("%s:\n got %s, %v\nwant %s", f, b, err, want)
			continue
		}
		var got payload
		if err := json.Unmarshal(b, &got, With(f)); err != nil {
			t.Errorf("%s: Unmarshal: %v", f, err)
			continue
		}
		want2 := samplePayload()
		if got.TTL != want2.TTL || *got.Ptr != *want2.Ptr || got.List[1] != want2.List[1] || got.Keys[time.Minute] != time.Hour {
			t.Errorf("%s: round trip = %+v", f, got)
		}
	}
}

// TestNanoMatchesV1 checks that Nano is byte-for-byte compatible with
// encoding/json (v1) in both directions.
func TestNanoMatchesV1(t *testing.T) {
	p := samplePayload()
	v1, err := jsonv1.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := json.Marshal(p, With(Nano))
	if err != nil {
		t.Fatal(err)
	}
	if string(v1) != string(v2) {
		t.Errorf("v1 and Nano differ:\nv1 %s\nv2 %s", v1, v2)
	}
	var fromV1 payload
	if err := json.Unmarshal(v1, &fromV1, With(Nano)); err != nil || fromV1.TTL != d90 {
		t.Errorf("Unmarshal v1 output = %v, %v", fromV1.TTL, err)
	}
}

func TestStringifyNumbers(t *testing.T) {
	b, err := json.Marshal(struct{ D time.Duration }{d90}, With(Sec), json.StringifyNumbers(true))
	if err != nil || string(b) != `{"D":"5400.5"}` {
		t.Errorf("got %s, %v", b, err)
	}
	var v struct{ D time.Duration }
	if err := json.Unmarshal(b, &v, With(Sec), json.StringifyNumbers(true)); err != nil || v.D != d90 {
		t.Errorf("Unmarshal = %v, %v", v.D, err)
	}
}

func TestUnmarshalStrict(t *testing.T) {
	tests := []struct {
		f       Format
		in      string
		wantErr string
	}{
		{Sec, `{"D":"5400.5"}`, "must be a JSON number"},        // string where number is required
		{ISO8601, `{"D":5400}`, "must be a JSON string"},        // number where string is required
		{ISO8601, `{"D":"1h30m"}`, "invalid ISO 8601 duration"}, // other format, no fallback
		{Units, `{"D":"PT1H"}`, "invalid duration"},             // other format, no fallback
		{ISO8601, `{"D":"P1D"}`, "nominal"},                     // nominal units
		{Nano, `{"D":true}`, "must be a JSON number"},           // wrong kind
		{Nano, `{"D":1e3}`, "invalid duration"},                 // exponent
		{ProtoJSON, `{"D":"1.5"}`, "invalid protobuf duration"}, // missing suffix
		{Sec, `{"D":99999999999}`, "value out of range"},        // overflow
	}
	for _, tt := range tests {
		var v struct{ D time.Duration }
		err := json.Unmarshal([]byte(tt.in), &v, With(tt.f))
		if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
			t.Errorf("%s %s: err = %v, want containing %q", tt.f, tt.in, err, tt.wantErr)
		}
		if err != nil && !strings.Contains(err.Error(), `"/D"`) {
			t.Errorf("%s %s: err %q does not include the JSON pointer", tt.f, tt.in, err)
		}
	}
}

func TestUnmarshalNullAndEscapes(t *testing.T) {
	v := struct{ D time.Duration }{D: time.Hour}
	if err := json.Unmarshal([]byte(`{"D":null}`), &v, With(ISO8601)); err != nil || v.D != 0 {
		t.Errorf("null: D = %v, %v", v.D, err)
	}
	if err := json.Unmarshal([]byte(`{"D":"PT1\u0048"}`), &v, With(ISO8601)); err != nil || v.D != time.Hour {
		t.Errorf("escaped: D = %v, %v", v.D, err)
	}
}

func TestPlainV2Fails(t *testing.T) {
	if _, err := json.Marshal(samplePayload()); err == nil {
		t.Fatal("encoding/json/v2 now has a default for time.Duration; revisit this package")
	}
}

func TestJoinWithOwnMarshalers(t *testing.T) {
	type ID string
	own := json.MarshalToFunc(func(enc *jsontext.Encoder, id ID) error {
		return enc.WriteToken(jsontext.String("id-" + string(id)))
	})
	opts := json.WithMarshalers(json.JoinMarshalers(own, Marshalers(Units)))
	b, err := json.Marshal(struct {
		ID ID
		D  time.Duration
	}{"x", time.Second}, opts)
	if err != nil || string(b) != `{"ID":"id-x","D":"1s"}` {
		t.Errorf("got %s, %v", b, err)
	}
}

func TestEncoderStreaming(t *testing.T) {
	var sb strings.Builder
	enc := jsontext.NewEncoder(&sb, With(ProtoJSON))
	for _, d := range []time.Duration{time.Second, 1500 * time.Millisecond} {
		if err := json.MarshalEncode(enc, d); err != nil {
			t.Fatal(err)
		}
	}
	if got := sb.String(); got != "\"1s\"\n\"1.500s\"\n" {
		t.Errorf("got %q", got)
	}
}

// TestConcurrentUse shares one options value between goroutines, as a
// package-level options variable would be. It only finds something when
// run with -race, which CI does.
func TestConcurrentUse(t *testing.T) {
	for _, f := range Formats() {
		opts := With(f)
		want, err := json.Marshal(samplePayload(), opts)
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				for range 50 {
					b, err := json.Marshal(samplePayload(), opts)
					if err != nil || string(b) != string(want) {
						t.Errorf("%s: Marshal = %s, %v; want %s", f, b, err, want)
						return
					}
					var got payload
					if err := json.Unmarshal(b, &got, opts); err != nil || got.TTL != d90 {
						t.Errorf("%s: Unmarshal = %v, %v; want %v", f, got.TTL, err, d90)
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
	for _, in := range []string{
		`{"d":5400500000000,"m":{"60000000000":3600000000000}}`, // as encoding/json (v1) writes it
		`{"d":"PT1H30M0.5S","m":{"PT1M":"PT1H"}}`,
		`{"d":"PT1H30M0.5S","m":{"60000000000":"PT1H"}}`, // mixed
	} {
		var v doc
		if err := json.Unmarshal([]byte(in), &v, opts); err != nil || v.D != d90 || v.M[time.Minute] != time.Hour {
			t.Errorf("Unmarshal(%s) = %+v, %v", in, v, err)
		}
	}
	// Only the first format is written.
	b, err := json.Marshal(doc{D: d90, M: map[time.Duration]time.Duration{time.Minute: time.Hour}}, opts)
	if want := `{"d":"PT1H30M0.5S","m":{"PT1M":"PT1H"}}`; err != nil || string(b) != want {
		t.Errorf("Marshal = %s, %v; want %s", b, err, want)
	}

	// Units and ProtoJSON overlap and agree.
	var d time.Duration
	if err := json.Unmarshal([]byte(`"1.500s"`), &d, With(Units, AlsoAccept(ProtoJSON))); err != nil || d != 1500*time.Millisecond {
		t.Errorf("Units+ProtoJSON: %v, %v", d, err)
	}
	// A numeric format is a JSON string with StringifyNumbers.
	if err := json.Unmarshal([]byte(`"5400.5"`), &d, With(Units, AlsoAccept(Sec)), json.StringifyNumbers(true)); err != nil || d != d90 {
		t.Errorf("Units+Sec stringified: %v, %v", d, err)
	}
	// Duplicates and nil options are harmless.
	err = json.Unmarshal([]byte(`"x"`), &d, With(ISO8601, AlsoAccept(ISO8601), nil))
	if err == nil || strings.Contains(err.Error(), "accepted") {
		t.Errorf("duplicates: err = %v, want the plain ISO 8601 error", err)
	}
}

func TestAlsoAcceptErrors(t *testing.T) {
	opts := With(ISO8601, AlsoAccept(Nano))
	tests := []struct {
		in      string
		wantErr error  // checked with errors.Is, if set
		wantMsg string // in the message
	}{
		{`{"D":"1h30m"}`, strconv.ErrSyntax, `invalid ISO 8601 duration "1h30m": invalid syntax (accepted: iso8601 or nano)`},
		{`{"D":"P1D"}`, ErrNominalUnits, "(accepted: iso8601 or nano)"},
		{`{"D":1e3}`, strconv.ErrSyntax, `invalid duration "1e3"`},
		{`{"D":"5400"}`, strconv.ErrSyntax, "invalid ISO 8601 duration"}, // Nano is a number outside object names
		{`{"D":true}`, nil, "iso8601 or nano duration must be a JSON string or number, got true"},
		{`{"D":[]}`, nil, "must be a JSON string or number, got ["},
	}
	for _, tt := range tests {
		var v struct{ D time.Duration }
		err := json.Unmarshal([]byte(tt.in), &v, opts)
		if err == nil || !strings.Contains(err.Error(), tt.wantMsg) || (tt.wantErr != nil && !errors.Is(err, tt.wantErr)) {
			t.Errorf("Unmarshal(%s) err = %v, want %q (%v)", tt.in, err, tt.wantMsg, tt.wantErr)
		}
	}
}

func TestAcceptDays(t *testing.T) {
	const day = 24 * time.Hour
	opts := With(ISO8601, AcceptDays())
	tests := []struct {
		in      string
		want    time.Duration
		wantErr error
	}{
		{`"P1D"`, day, nil},
		{`"p2d"`, 2 * day, nil},
		{`"P1W"`, 7 * day, nil},
		{`"P1W2D"`, 9 * day, nil},
		{`"P1DT12H"`, 36 * time.Hour, nil},
		{`"-P1D"`, -day, nil},
		{`"P106751DT23H47M16.854775807S"`, maxD, nil},
		{`"P106752D"`, 0, strconv.ErrRange},
		{`"P1Y"`, 0, ErrNominalUnits},
		{`"P1M"`, 0, ErrNominalUnits},
		{`"P1M1D"`, 0, ErrNominalUnits},
		{`"P1.5D"`, 0, strconv.ErrSyntax}, // days take no fraction
	}
	for _, tt := range tests {
		var d time.Duration
		err := json.Unmarshal([]byte(tt.in), &d, opts)
		switch {
		case tt.wantErr == nil && (err != nil || d != tt.want):
			t.Errorf("Unmarshal(%s) = %v, %v; want %v", tt.in, d, err, tt.want)
		case tt.wantErr != nil && !errors.Is(err, tt.wantErr):
			t.Errorf("Unmarshal(%s) error = %v, want %v", tt.in, err, tt.wantErr)
		}
	}

	var d time.Duration
	if err := json.Unmarshal([]byte(`"P1D"`), &d, With(ISO8601)); !errors.Is(err, ErrNominalUnits) {
		t.Errorf("without AcceptDays: err = %v, want ErrNominalUnits", err)
	}
	if b, err := json.Marshal(36*time.Hour, opts); err != nil || string(b) != `"PT36H"` {
		t.Errorf("Marshal = %s, %v; days are never written", b, err)
	}
	// ISO 8601 need not be the written format.
	if err := json.Unmarshal([]byte(`"P1D"`), &d, With(Nano, AlsoAccept(ISO8601), AcceptDays())); err != nil || d != day {
		t.Errorf("Nano+ISO8601: %v, %v; want 24h", d, err)
	}
	// Other formats are not affected.
	if err := json.Unmarshal([]byte(`"1h"`), &d, With(Units, AcceptDays())); err != nil || d != time.Hour {
		t.Errorf("Units: %v, %v; want 1h", d, err)
	}
}

func TestUnmarshalSyntaxError(t *testing.T) {
	var v struct{ D time.Duration }
	err := json.Unmarshal([]byte(`{"D":tru}`), &v, With(Nano))
	var serr *jsontext.SyntacticError
	if !errors.As(err, &serr) {
		t.Errorf("err = %v, want a *jsontext.SyntacticError", err)
	}
}

func TestUnmarshalInvalidUTF8(t *testing.T) {
	// With AllowInvalidUTF8 the decoder passes invalid bytes through;
	// they must still be rejected, with or without escapes in the string.
	for _, in := range []string{"\"PT1H\xff\"", "\"PT1\\u0048\xff\""} {
		var d time.Duration
		if err := json.Unmarshal([]byte(in), &d, With(ISO8601), jsontext.AllowInvalidUTF8(true)); err == nil {
			t.Errorf("Unmarshal(%q) = %v, want error", in, d)
		}
	}
}

// FuzzUnmarshal decodes arbitrary JSON with every format. Decoding must
// never panic, and whatever decodes must survive a second round trip
// unchanged.
func FuzzUnmarshal(f *testing.F) {
	for _, s := range []string{
		`{"d":"PT1H30M0.5S","p":null,"l":["1s","-1ms"],"m":{"60":3600}}`,
		`{"d":5400.5,"l":[1,-0.001],"m":{"PT1M":"PT1H"}}`,
		`{"d":"1:30:00.5","m":{"0:01:00":"1:00:00"}}`,
		`{"d":"5400.500s","p":"1s"}`,
		`{"d":"1h30m","l":[null]}`,
		`[1,2,3]`, `null`, `{"d":"PT1H"}`,
	} {
		f.Add([]byte(s))
	}
	type doc struct {
		D time.Duration                   `json:"d"`
		P *time.Duration                  `json:"p"`
		L []time.Duration                 `json:"l"`
		M map[time.Duration]time.Duration `json:"m"`
	}
	// Every format alone, and every string format that also accepts Nano.
	type config struct {
		name string
		opts json.Options
	}
	var configs []config
	for _, fm := range Formats() {
		configs = append(configs, config{string(fm), json.JoinOptions(With(fm), json.Deterministic(true))})
		if !fm.IsNumeric() {
			opts := json.JoinOptions(With(fm, AlsoAccept(Nano)), json.Deterministic(true))
			configs = append(configs, config{string(fm) + "+nano", opts})
		}
	}
	configs = append(configs, config{"iso8601+days", json.JoinOptions(With(ISO8601, AcceptDays()), json.Deterministic(true))})
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
			if b2, _ := json.Marshal(w, c.opts); string(b1) != string(b2) {
				t.Fatalf("%s: round trip changed\n%s\n%s", c.name, b1, b2)
			}
		}
	})
}
