// Copyright 2026 Patrick Scheid. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonduration

import (
	"encoding/json/v2"
	"testing"
	"time"
)

func BenchmarkMarshal(b *testing.B) {
	for _, f := range Formats() {
		b.Run(string(f), func(b *testing.B) {
			opts := With(f)
			v := struct{ D time.Duration }{d90}
			b.ReportAllocs()
			for b.Loop() {
				if _, err := json.Marshal(v, opts); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkUnmarshal(b *testing.B) {
	for _, f := range Formats() {
		in, err := json.Marshal(struct{ D time.Duration }{d90}, With(f))
		if err != nil {
			b.Fatal(err)
		}
		b.Run(string(f), func(b *testing.B) {
			opts := With(f)
			var v struct{ D time.Duration }
			b.ReportAllocs()
			for b.Loop() {
				if err := json.Unmarshal(in, &v, opts); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkMarshalInt64 is the baseline: the same struct with a plain
// int64, which v2 encodes without any caller-specified marshaler.
func BenchmarkMarshalInt64(b *testing.B) {
	v := struct{ D int64 }{int64(d90)}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := json.Marshal(v); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParse(b *testing.B) {
	for _, f := range Formats() {
		s := f.Format(d90)
		b.Run(string(f), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := f.Parse(s); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
