// Copyright 2026 Patrick Scheid. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonduration

// The wrapper types, one per format, are generated into wrapper_types.go.
//go:generate go run gen_wrapper_types.go

// decoders holds a single-format decoder for each format, for the
// UnmarshalJSONFrom methods of the wrapper types.
var decoders = func() map[Format]*decoder {
	m := make(map[Format]*decoder)
	for _, f := range Formats() {
		m[f] = newDecoder(f, nil)
	}
	return m
}()
