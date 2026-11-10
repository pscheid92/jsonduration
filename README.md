# jsonduration

[![CI](https://github.com/pscheid92/jsonduration/actions/workflows/ci.yml/badge.svg)](https://github.com/pscheid92/jsonduration/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/pscheid92/jsonduration.svg)](https://pkg.go.dev/github.com/pscheid92/jsonduration)

JSON formats for `time.Duration` in Go's `encoding/json/v2`.

`encoding/json/v2` can't encode a `time.Duration` on its own: it has no default representation
([golang/go#71631](https://go.dev/issue/71631)), and the `format` struct tag that used to pick one was removed in Go
1.27 ([golang/go#79071](https://go.dev/issue/79071)). This package lets you choose one.

```sh
go get github.com/pscheid92/jsonduration
```

It requires Go 1.27 or later, has no dependencies outside the standard library, and supports `encoding/json/v2` only.
With `encoding/json` (v1), use v2 or a field type of your own; this package is not tested with v1.
Until v1.0.0 the API may still change.

## Usage

Without this package, `json.Marshal` of a `time.Duration` fails with `no default representation`. Create one option
with `With` and pass it to each `json.Marshal` and `json.Unmarshal`. It applies one format to every `time.Duration` in
that call: in fields, pointers, slices, arrays, and map keys and values.

```go
package main

import (
	"encoding/json/v2"
	"fmt"
	"time"

	"github.com/pscheid92/jsonduration"
)

type Config struct {
	Timeout time.Duration   `json:"timeout"`
	Retries []time.Duration `json:"retries"`
}

// Options are safe for concurrent use.
var opts = jsonduration.With(jsonduration.ISO8601)

func main() {
	cfg := Config{Timeout: 90 * time.Second, Retries: []time.Duration{time.Second, 5 * time.Second}}

	b, err := json.Marshal(cfg, opts)
	fmt.Println(string(b), err) // {"timeout":"PT1M30S","retries":["PT1S","PT5S"]} <nil>

	err = json.Unmarshal([]byte(`{"timeout":"PT2M"}`), &cfg, opts)
	fmt.Println(cfg.Timeout, err) // 2m0s <nil>
}
```

`encoding/json/v2` is part of the standard library since Go 1.27, with no `GOEXPERIMENT` setting. Without decoding
options, `With` returns the same options on every call and allocates nothing. With `AlsoAccept` or `AcceptDays` it
builds new ones, so keep those in a variable.

If you already pass `json.WithMarshalers` or `json.WithUnmarshalers`, don't add `With` next to them: a second
`WithMarshalers` option replaces the first, without an error. Join yours with the package's instead:

```go
opts := json.JoinOptions(
	json.WithMarshalers(json.JoinMarshalers(yours, jsonduration.Marshalers(jsonduration.Units))),
	json.WithUnmarshalers(json.JoinUnmarshalers(yoursToo, jsonduration.Unmarshalers(jsonduration.Units))),
)
```

If you can't pass options, for example because other code encodes your types, or if fields need different formats,
give each field a wrapper type. The wrapper types are listed in the table below:

```go
type Config struct {
	Timeout jsonduration.ISO8601Duration `json:"timeout"` // "PT1M30S"
	Backoff jsonduration.SecDuration     `json:"backoff"` // 1.5
}

cfg := Config{Timeout: jsonduration.ISO8601Duration{Duration: 90 * time.Second}}
```

A wrapper type embeds `time.Duration`, so `cfg.Timeout.Minutes()` keeps working, but where a `time.Duration` is
expected you write `cfg.Timeout.Duration`. To keep plain `time.Duration` fields in a struct that others encode, see
[Your own types](#your-own-types).

## Formats

| Format    | Name      | Wrapper type      | JSON type | 1h30m0.5s becomes |
|-----------|-----------|-------------------|-----------|-------------------|
| `Units`   | `units`   | `UnitsDuration`   | string    | `"1h30m0.5s"`     |
| `Sec`     | `sec`     | `SecDuration`     | number    | `5400.5`          |
| `Milli`   | `milli`   | `MilliDuration`   | number    | `5400500`         |
| `Micro`   | `micro`   | `MicroDuration`   | number    | `5400500000`      |
| `Nano`    | `nano`    | `NanoDuration`    | number    | `5400500000000`   |
| `ISO8601` | `iso8601` | `ISO8601Duration` | string    | `"PT1H30M0.5S"`   |

Use `ISO8601` for APIs that other languages read, `Units` for configuration that people write, `Sec` or `Milli` where
a number is expected, and `Nano` for data shared with `encoding/json` (v1), which writes the same.

Numeric formats are exact and write fractions where needed, so `Milli` writes 1.5ms as `1.5`. A decoder that reads
JSON numbers as `float64`, such as JavaScript's `JSON.parse`, can lose the nanoseconds of long durations.

## Common tasks

### Replacing the `format` tag

Remove the tag option, and choose the format in one of the two ways above:

```go
// Before Go 1.27
Timeout time.Duration `json:"timeout,format:iso8601"`

// With jsonduration.With(jsonduration.ISO8601) passed to json.Marshal and json.Unmarshal
Timeout time.Duration `json:"timeout"`

// Or without options
Timeout jsonduration.ISO8601Duration `json:"timeout"`
```

Stored data stays readable: each format writes what the tag option of the same name wrote, and a test checks this
against the tag option's own code. Decoding differs in these ways:

| Input | The tag option | jsonduration |
|---|---|---|
| `-0` in a numeric format | out of range | zero |
| A calendar unit with a count of zero, as in `"P0D"` | rejected | read |
| A fraction that is not all digits, as in `"PT1.e5H"` | one hour | rejected |
| Fractions in `Units`, and of hours and minutes in `ISO8601` | rounded through `float64` | exact, to the nanosecond |
| The `string` tag option on `Units` and `ISO8601` | rejected | no effect |

### Reading more than one format

```go
// Stored data holds encoding/json (v1) nanoseconds; new data is ISO 8601.
opts := jsonduration.With(jsonduration.ISO8601, jsonduration.AlsoAccept(jsonduration.Nano))
```

Both `90000000000` and `"PT1M30S"` decode to 1m30s, and everything is written as ISO 8601. You can accept several
string formats, but only one numeric format, because `90` could be seconds or nanoseconds.

When several services share the data, let every reader accept the new format before any writer switches to it. A
service still on `encoding/json` (v1) cannot read ISO 8601 at all.

### ISO 8601 days

`ISO8601` rejects days, weeks, months, and years, because a calendar day is not always 24 hours long. If the other side
means exactly 24 hours by a day, as Python's Pydantic and .NET's `XmlConvert` do, add `AcceptDays`:

```go
opts := jsonduration.With(jsonduration.ISO8601, jsonduration.AcceptDays())
// "P1DT12H" decodes to 36h
```

Without `AcceptDays`, `"P1D"` fails with `calendar units have no fixed length`. Months and years stay rejected, and
output never uses days. A count of zero, as in `"P0D"` or `"P0Y0M0DT1H30M0S"`, is
always read.

### Your own types

The wrapper types take no options, so `ISO8601Duration` rejects `"P1DT12H"`. If a field needs `AcceptDays` or
`AlsoAccept`, write your own wrapper type with the imports `encoding/json/v2` and `encoding/json/jsontext`:

```go
var daysOptions = jsonduration.With(jsonduration.ISO8601, jsonduration.AcceptDays())

type DaysDuration struct{ time.Duration }

func (d DaysDuration) MarshalJSONTo(enc *jsontext.Encoder) error {
	return json.MarshalEncode(enc, d.Duration, daysOptions)
}

func (d *DaysDuration) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return json.UnmarshalDecode(dec, &d.Duration, daysOptions)
}
```

The same two methods give a named type such as `type TTL time.Duration` a format, since `With` does not reach it:

```go
var unitsOptions = jsonduration.With(jsonduration.Units)

type TTL time.Duration

func (t TTL) MarshalJSONTo(enc *jsontext.Encoder) error {
	return json.MarshalEncode(enc, time.Duration(t), unitsOptions)
}

func (t *TTL) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return json.UnmarshalDecode(dec, (*time.Duration)(t), unitsOptions)
}
```

A struct that others encode without options can keep plain `time.Duration` fields with the same two methods on the
struct itself, as [`RetryPolicy` in the examples](example_own_types_test.go) shows. Others must then use it as a named
field, `Retry RetryPolicy`, and not embed it: an embedded struct passes its methods on, and the outer struct then
encodes as the inner one alone, without its other fields.

### Choosing the format at run time

`Format` works with `flag.TextVar` and in JSON configuration. Names must match the table exactly, in lower case, so
`ISO8601` is not a name:

```go
var format jsonduration.Format
flag.TextVar(&format, "duration-format", jsonduration.ISO8601, "how durations are written")
flag.Parse()
opts := jsonduration.With(format)
```

A `Format` is its name, which is what lets a flag or a configuration hold one. So a `Format` that names nothing can
exist, like `time.Month(13)`, and using it is a programming error: `With` panics on an invalid `Format`, which
includes the empty one. Check a `Format` from a plain string first:

```go
format := jsonduration.Format(os.Getenv("DURATION_FORMAT"))
if !format.Valid() {
	log.Fatalf("DURATION_FORMAT=%q, want one of %v", format, jsonduration.Formats())
}
```

### Without JSON

```go
jsonduration.ISO8601.Format(90 * time.Second) // "PT1M30S"
d, err := jsonduration.Sec.Parse("5400.5")    // 1h30m0.5s
```

## Good to know

- Every error about a bad duration matches `jsonduration.ErrInvalidDuration`, and its cause: `strconv.ErrSyntax`,
  `strconv.ErrRange`, `ErrCalendarUnits`, or `ErrWrongType`. The `json.SemanticError` around it says where it is,
  in its `JSONPointer`. Malformed JSON is a `jsontext.SyntacticError` instead.
- JSON `null` sets a duration to zero, and a `*time.Duration` to nil.
- To leave out zero durations, use `omitzero`. In v2, `omitempty` keeps them.
- Decoding checks the format, not the value: `"-1s"` and `"0s"` are read. Check ranges after decoding.
- Numeric formats reject exponents, as in `1e-05`, which some encoders such as Swift's write for small numbers.
  That is on purpose: decoding is strict, and exponents would add a second spelling of every number for a rare case.
- Digits below a nanosecond are truncated, not rounded. No format rounds to whole units: `Milli` writes `1.234567`.
- The largest duration is about 292 years, `PT2562047H47M16.854775807S`; anything larger is `strconv.ErrRange`.
- `ISO8601` takes one sign, `-` or `+`, only in front, as in `-PT30S`. It rejects Java's negative durations, such as
  `PT-30S`, as the `format` tag of `encoding/json/v2` did. It reads designators in either case and a decimal comma,
  so `pt1,5s` is 1.5s. A count of zero must have no fraction: `"P0D"` is read, `"P0.0D"` is not. Output is always the
  shortest form, so `"P0DT2H"` comes back as `"PT2H"`.
- `Units` reads what `time.ParseDuration` reads. So it reads protobuf's `"3600s"` but writes `"1h0m0s"`, and it does
  not read Rust humantime's `"1h 30m"` or PostgreSQL's `"01:30:00"`. PostgreSQL writes ISO 8601 with
  `intervalstyle = iso_8601`.
- Decoding into `any` gives the JSON as it is, such as the string `"PT1S"`, since the type is unknown there.
- Maps are written in random order, as for every map in `encoding/json/v2`, unless you pass `json.Deterministic(true)`.
- If nanoseconds are all you need, `encoding/json` has an option for the v2 functions:
  `jsonv1.FormatDurationAsNano(true)`.

## Future

Go may add its own way to choose a format for a type, see [golang/go#71664](https://go.dev/issue/71664) and
[golang/go#81013](https://go.dev/issue/81013). If that happens, prefer it. A final release of this package will then
point to it.

## How it is tested

Every codec is compared with an independent implementation: the removed `format` tag option's own code, a second
parser built from regular expressions and `math/big`, and `time.ParseDuration`. Each comparison is also a fuzz target.
On top of that come mutation testing every night, 100% statement coverage, and durations as real producers write
them. [TESTING.md](TESTING.md) has the details.

## How fast it is

jsonduration allocates nothing of its own: writing and reading a duration allocates what an `int64` in the same place
does, and `With` without decoding options allocates nothing. Tests enforce both. In a struct with many fields,
wrapper types are faster than options. [BENCHMARKS.md](BENCHMARKS.md) has rough timings and the reasons.

## Development

Needs [just](https://just.systems) and [golangci-lint](https://golangci-lint.run). Run `just` to list all recipes.

```sh
just check        # go fix, gofmt, go mod tidy, vet, lint, actionlint, tests with -race, compat, govulncheck
just fuzz         # every fuzz target for 30s
just compat-fuzz  # the fuzz targets of the comparison with the format tag option, for 30s each
just mutate       # mutation testing with gremlins, which takes a few minutes
```

## License

BSD 3-Clause, see [LICENSE](LICENSE). Parts of the codecs are adapted from the Go standard library, whose notice is
in [NOTICE](NOTICE).
