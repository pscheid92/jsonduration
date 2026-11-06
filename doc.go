// Copyright 2026 Patrick Scheid. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package jsonduration gives [time.Duration] a JSON representation in
// encoding/json/v2.
//
// encoding/json/v2 has no default representation for time.Duration
// (https://go.dev/issue/71631): marshaling or unmarshaling one returns an
// error. The `format` struct tag that used to select one was removed in
// Go 1.27 (https://go.dev/issue/79071), and its replacement depends on typed
// struct tags (https://go.dev/issue/74472). This package fills the gap in
// two ways. It is for encoding/json/v2 only.
//
// # One format for a whole call
//
// [With] returns options that apply one format to every time.Duration in a
// call: struct fields, pointers, slices, arrays, map keys and values, and
// values held in an any when marshaling. Your types stay unchanged.
//
//	opts := jsonduration.With(jsonduration.ISO8601)
//	b, err := json.Marshal(v, opts)  // {"ttl":"PT1H30M"}
//	err = json.Unmarshal(b, &v, opts)
//
// # One format per field
//
// Each format has a wrapper type, such as [ISO8601Duration] or
// [SecDuration], that embeds a time.Duration and encodes itself through
// the encoding/json/v2 interfaces MarshalerTo and UnmarshalerFrom. Wrapper
// types need no options, and [With] does not change their format.
//
//	type Config struct {
//		Timeout jsonduration.ISO8601Duration `json:"timeout"`
//		Backoff jsonduration.SecDuration     `json:"backoff"`
//	}
//
// # Formats
//
// The formats are the six that encoding/json/v2 offered through `format`,
// with the same names and identical output: [Units], [Sec], [Milli],
// [Micro], [Nano] and [ISO8601]. [Nano] is byte-for-byte compatible with
// encoding/json (v1).
//
// A [Format] can come from a JSON configuration or from flags: it
// implements [encoding.TextUnmarshaler] and rejects anything but the exact
// names. Check a Format converted from a plain string with [Format.Valid].
//
// # Strictness
//
// Decoding accepts only the configured format. To read data written in
// other formats as well, for example while migrating, name them with
// [AlsoAccept]. [ISO8601] input with nominal units (years, months, weeks,
// days) is rejected with [ErrNominalUnits], since those have no fixed
// length; where a producer means 24 hours by a day, [AcceptDays] reads days
// and weeks. Fractional digits below one nanosecond are truncated toward
// zero, exactly, in every format. Errors wrap [strconv.ErrSyntax],
// [strconv.ErrRange] or [ErrNominalUnits].
//
// JSON null sets a duration to zero, as encoding/json/v2 does for other
// types.
//
// # Good to know
//
// Named types such as `type TTL time.Duration` are distinct types and are
// not affected by [With]. To omit zero durations, use the `omitzero` tag
// option: in v2, `omitempty` only omits values that encode as empty JSON.
// The `string` tag option quotes numeric formats, like
// json.StringifyNumbers, and has no effect on string formats.
package jsonduration
