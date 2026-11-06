// Copyright 2026 Patrick Scheid. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonduration

import (
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

// newWrapper returns a pointer to a new value of the type of like,
// holding d.
func newWrapper(like any, d time.Duration) any {
	p := reflect.New(reflect.TypeOf(like))
	p.Elem().Field(0).SetInt(int64(d))
	return p.Interface()
}

// unwrap returns the duration a pointer from newWrapper holds.
func unwrap(p any) time.Duration {
	return time.Duration(reflect.ValueOf(p).Elem().Field(0).Int())
}

// TestWrapperTypes checks that each wrapper type encodes exactly as With
// does for its format, which also catches a mix-up between them.
func TestWrapperTypes(t *testing.T) {
	if len(wrappers) != len(Formats()) {
		t.Fatalf("%d wrapper types for %d formats", len(wrappers), len(Formats()))
	}
	for f, like := range wrappers {
		for _, d := range []time.Duration{0, d90, -time.Nanosecond, maxD, minD} {
			want, err := json.Marshal(d, With(f))
			if err != nil {
				t.Fatal(err)
			}
			v := reflect.ValueOf(newWrapper(like, d)).Elem().Interface()
			if b, err := json.Marshal(v); err != nil || string(b) != string(want) {
				t.Errorf("%T %s: Marshal = %s, %v; want %s", v, d, b, err, want)
			}
			p := newWrapper(like, 42)
			if err := json.Unmarshal(want, p); err != nil || unwrap(p) != d {
				t.Errorf("%T %s: Unmarshal(%s) = %s, %v", v, d, want, unwrap(p), err)
			}
		}
	}
}

func TestWrapperErrors(t *testing.T) {
	for f, like := range wrappers {
		p := newWrapper(like, time.Hour)
		if err := json.Unmarshal([]byte(`"x"`), p); err == nil {
			t.Errorf("%s: Unmarshal(\"x\") succeeded", f)
		}
		if unwrap(p) != time.Hour {
			t.Errorf("%s: failed decoding changed the duration to %s", f, unwrap(p))
		}
	}
	// Decoding is as strict as with With.
	for in, want := range map[string]string{
		`{"I":90}`:       "must be a JSON string",
		`{"S":"5400.5"}`: "must be a JSON number",
	} {
		var v struct {
			I ISO8601Duration
			S SecDuration
		}
		if err := json.Unmarshal([]byte(in), &v); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Unmarshal(%s) err = %v, want %q", in, err, want)
		}
	}
}

// TestWrapperFields mixes formats in one struct, which needs no options.
func TestWrapperFields(t *testing.T) {
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
		t.Errorf("Marshal = %s, %v; want %s", b, err, want)
	}
	var got config
	if err := json.Unmarshal([]byte(want), &got); err != nil || !reflect.DeepEqual(got, c) {
		t.Errorf("Unmarshal = %+v, %v; want %+v", got, err, c)
	}
}

// TestWrapperKeepsFormat checks that With does not reach into a wrapper.
func TestWrapperKeepsFormat(t *testing.T) {
	type mixed struct {
		W ISO8601Duration
		D time.Duration
	}
	b, err := json.Marshal(mixed{ISO8601Duration{time.Minute}, time.Minute}, With(Units))
	if want := `{"W":"PT1M","D":"1m0s"}`; err != nil || string(b) != want {
		t.Errorf("Marshal = %s, %v; want %s", b, err, want)
	}
}

// TestWrapperNull: JSON null sets a wrapper to zero, as encoding/json/v2
// does for other types.
func TestWrapperNull(t *testing.T) {
	v := struct{ D ISO8601Duration }{ISO8601Duration{time.Hour}}
	if err := json.Unmarshal([]byte(`{"D":null}`), &v); err != nil || v.D.Duration != 0 {
		t.Errorf("D = %s, %v; want 0", v.D, err)
	}
}

func TestWrapperStringifyNumbers(t *testing.T) {
	v := struct{ B SecDuration }{SecDuration{1500 * time.Millisecond}}
	b, err := json.Marshal(v, json.StringifyNumbers(true))
	if want := `{"B":"1.5"}`; err != nil || string(b) != want {
		t.Errorf("Marshal = %s, %v; want %s", b, err, want)
	}
}
