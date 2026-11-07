package jsonduration

import (
	"bytes"
	"encoding/json/v2"
	"reflect"
	"strings"
	"testing"
	"time"
)

// wrappers maps each format to its wrapper type.
var wrappers = map[Format]any{
	Units:   UnitsDuration{},
	Sec:     SecDuration{},
	Milli:   MilliDuration{},
	Micro:   MicroDuration{},
	Nano:    NanoDuration{},
	ISO8601: ISO8601Duration{},
}

// newWrapper returns a pointer to a new value of the type of like, holding d.
func newWrapper(like any, d time.Duration) any {
	p := reflect.New(reflect.TypeOf(like))
	p.Elem().Field(0).SetInt(int64(d))
	return p.Interface()
}

// wrap returns a value of the type of like, holding d.
func wrap(like any, d time.Duration) any {
	p := newWrapper(like, d)
	return reflect.ValueOf(p).Elem().Interface()
}

// unwrap returns the duration that a pointer from newWrapper holds.
func unwrap(p any) time.Duration {
	field := reflect.ValueOf(p).Elem().Field(0)
	return time.Duration(field.Int())
}

// TestWrapperTypes checks that each wrapper type encodes exactly as With does for its format,
// which also catches a mix-up between them.
func TestWrapperTypes(t *testing.T) {
	if len(wrappers) != len(Formats()) {
		t.Fatalf("%d wrapper types for %d formats", len(wrappers), len(Formats()))
	}

	durations := []time.Duration{0, d90, -time.Nanosecond, maxD, minD}

	for f, like := range wrappers {
		t.Run(string(f), func(t *testing.T) {
			opts := With(f)

			for _, d := range durations {
				want, err := json.Marshal(d, opts)
				if err != nil {
					t.Fatal(err)
				}

				v := wrap(like, d)
				if b, err := json.Marshal(v); err != nil || !bytes.Equal(b, want) {
					t.Errorf("Marshal(%T(%s)) = %s, %v, want %s", v, d, b, err, want)
				}

				p := newWrapper(like, 42)
				if err := json.Unmarshal(want, p); err != nil || unwrap(p) != d {
					t.Errorf("Unmarshal(%s) into %T = %s, %v, want %s", want, v, unwrap(p), err, d)
				}
			}
		})
	}
}

func TestWrapperKeepsValueOnError(t *testing.T) {
	for f, like := range wrappers {
		t.Run(string(f), func(t *testing.T) {
			p := newWrapper(like, time.Hour)

			if err := json.Unmarshal([]byte(`"x"`), p); err == nil {
				t.Errorf(`Unmarshal("x") succeeded, want an error`)
			}

			if unwrap(p) != time.Hour {
				t.Errorf(`Unmarshal("x") changed the duration to %s`, unwrap(p))
			}
		})
	}
}

func TestWrapperIsStrict(t *testing.T) {
	type config struct {
		I ISO8601Duration
		S SecDuration
	}

	tests := map[string]string{
		`{"I":90}`:       "want a JSON string",
		`{"S":"5400.5"}`: "want a JSON number",
	}

	for in, want := range tests {
		var v config
		err := json.Unmarshal([]byte(in), &v)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Unmarshal(%s) error = %v, want one containing %q", in, err, want)
		}
	}
}

func TestWrappersMixFormatsWithoutOptions(t *testing.T) {
	type config struct {
		Timeout ISO8601Duration                  `json:"timeout"`
		Backoff SecDuration                      `json:"backoff"`
		Limits  map[ISO8601Duration]NanoDuration `json:"limits"`
		Steps   map[SecDuration]UnitsDuration    `json:"steps"`
		Delay   *UnitsDuration                   `json:"delay,omitzero"`
	}

	c := config{
		Timeout: ISO8601Duration{90 * time.Second},
		Backoff: SecDuration{1500 * time.Millisecond},
		Limits:  map[ISO8601Duration]NanoDuration{{time.Minute}: {time.Second}},
		Steps:   map[SecDuration]UnitsDuration{{1500 * time.Millisecond}: {time.Hour}},
	}

	const want = `{"timeout":"PT1M30S","backoff":1.5,"limits":{"PT1M":1000000000},"steps":{"1.5":"1h0m0s"}}`

	b, err := json.Marshal(c)
	if err != nil || string(b) != want {
		t.Errorf("Marshal = %s, %v, want %s", b, err, want)
	}

	var got config
	err = json.Unmarshal([]byte(want), &got)
	if err != nil || !reflect.DeepEqual(got, c) {
		t.Errorf("Unmarshal(%s) = %+v, %v, want %+v", want, got, err, c)
	}
}

func TestWithLeavesWrappersAlone(t *testing.T) {
	type mixed struct {
		W ISO8601Duration
		D time.Duration
	}

	opts := With(Units)
	v := mixed{W: ISO8601Duration{time.Minute}, D: time.Minute}
	const want = `{"W":"PT1M","D":"1m0s"}`

	b, err := json.Marshal(v, opts)
	if err != nil || string(b) != want {
		t.Errorf("Marshal = %s, %v, want %s", b, err, want)
	}
}

func TestWrapperNull(t *testing.T) {
	type config struct {
		D ISO8601Duration
	}

	v := config{D: ISO8601Duration{time.Hour}}

	err := json.Unmarshal([]byte(`{"D":null}`), &v)
	if err != nil || v.D.Duration != 0 {
		t.Errorf("Unmarshal(null) = %s, %v, want 0", v.D, err)
	}
}

func TestWrapperStringifyNumbers(t *testing.T) {
	type config struct {
		B SecDuration
	}

	stringify := json.StringifyNumbers(true)
	v := config{B: SecDuration{1500 * time.Millisecond}}
	const want = `{"B":"1.5"}`

	b, err := json.Marshal(v, stringify)
	if err != nil || string(b) != want {
		t.Errorf("Marshal = %s, %v, want %s", b, err, want)
	}
}
