// Package compat checks jsonduration against the `format` tag option that encoding/json/v2 removed in Go 1.27.
//
// The standard library still holds the code of that option, and the ExperimentalSupportFormatTag option of
// github.com/go-json-experiment/json turns it back on. The tests compare each format with the tag option of the same
// name, so that the claim that jsonduration replaces it rests on the original code, not on a description of it.
//
// It is a module of its own, so that jsonduration itself keeps no dependencies. It holds only tests.
package compat
