# jsonduration

[![CI](https://github.com/pscheid92/jsonduration/actions/workflows/ci.yml/badge.svg)](https://github.com/pscheid92/jsonduration/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/pscheid92/jsonduration.svg)](https://pkg.go.dev/github.com/pscheid92/jsonduration)
[![Coverage](https://img.shields.io/endpoint?url=https%3A%2F%2Fraw.githubusercontent.com%2Fpscheid92%2Fjsonduration%2Fbadges%2Fcoverage.json)](https://github.com/pscheid92/jsonduration/actions/workflows/ci.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/pscheid92/jsonduration/badge)](https://scorecard.dev/viewer/?uri=github.com/pscheid92/jsonduration)

JSON formats for `time.Duration` in Go's `encoding/json/v2`.

`encoding/json/v2` can't encode a `time.Duration` on its own: it has no default representation
([golang/go#71631](https://go.dev/issue/71631)), and the `format` struct tag that used to pick one was removed in Go
1.27 ([golang/go#79071](https://go.dev/issue/79071)). This package lets you choose one.

```sh
go get github.com/pscheid92/jsonduration
```

It requires Go 1.27 or later, has no dependencies outside the standard library, and supports `encoding/json/v2` only.
Until v1.0.0 the API may still change.

## Usage

There are two ways to choose a format:

- **One option per call**, if you call `json.Marshal` and `json.Unmarshal` yourself. It covers every `time.Duration` in
  the call, and your types stay as they are.
- **One wrapper type per field**, if you can't pass options, for example because another library encodes your types, or
  if fields need different formats.

### One option per call

```go
package main

import (
	"encoding/json/v2"
	"fmt"
	"log"
	"time"

	"github.com/pscheid92/jsonduration"
)

type Config struct {
	Timeout time.Duration   `json:"timeout"`
	Retries []time.Duration `json:"retries"`
}

func main() {
	cfg := Config{
		Timeout: 90 * time.Second,
		Retries: []time.Duration{time.Second, 5 * time.Second},
	}

	opts := jsonduration.With(jsonduration.ISO8601)

	b, err := json.Marshal(cfg, opts)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(b)) // {"timeout":"PT1M30S","retries":["PT1S","PT5S"]}

	var readConfig Config
	if err := json.Unmarshal([]byte(`{"timeout":"PT2M"}`), &readConfig, opts); err != nil {
		log.Fatal(err)
	}
	fmt.Println(readConfig.Timeout) // 2m0s
}
```

The format applies to fields, pointers, slices, arrays, map keys and values, and values in an `any` when marshaling.
Create the options once and share them: they are safe for concurrent use.

### One wrapper type per field

```go
package main

import (
	"encoding/json/v2"
	"fmt"
	"log"
	"time"

	"github.com/pscheid92/jsonduration"
)

type Config struct {
	Timeout jsonduration.ISO8601Duration `json:"timeout"`
	Backoff jsonduration.SecDuration     `json:"backoff"`
}

func main() {
	cfg := Config{
		Timeout: jsonduration.ISO8601Duration{Duration: 90 * time.Second},
		Backoff: jsonduration.SecDuration{Duration: 1500 * time.Millisecond},
	}

	b, err := json.Marshal(cfg)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(b)) // {"timeout":"PT1M30S","backoff":1.5}
}
```

Wrapper types embed `time.Duration`, so its methods keep working, as in `cfg.Timeout.Minutes()` or
`time.Sleep(cfg.Backoff.Duration)`.

A wrapper type writes and reads only its own format, and `AlsoAccept` and `AcceptDays` do not change it. If a field
needs one of them, write a wrapper type of your own, which takes a few lines:

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

## Formats

| Format      | Name        | Wrapper type        | JSON type | 1h30m0.5s becomes |
|-------------|-------------|---------------------|-----------|-------------------|
| `Units`     | `units`     | `UnitsDuration`     | string    | `"1h30m0.5s"`     |
| `Sec`       | `sec`       | `SecDuration`       | number    | `5400.5`          |
| `Milli`     | `milli`     | `MilliDuration`     | number    | `5400500`         |
| `Micro`     | `micro`     | `MicroDuration`     | number    | `5400500000`      |
| `Nano`      | `nano`      | `NanoDuration`      | number    | `5400500000000`   |
| `ISO8601`   | `iso8601`   | `ISO8601Duration`   | string    | `"PT1H30M0.5S"`   |

- These are the six formats the `format` tag offered before Go 1.27, and they write the same JSON as they did then.
- `Nano` writes what `encoding/json` (v1) writes.
- Numeric formats are exact, with no floating-point rounding.
- Decoders that read JSON numbers as `float64`, such as JavaScript's `JSON.parse`, can lose the nanoseconds of long
  durations in every numeric format, in `Nano` beyond about 104 days. So can the `jsontext.CanonicalizeRawInts` and
  `CanonicalizeRawFloats` options.

As a rule of thumb, use `ISO8601` for APIs that other languages read, `Units` for configuration that people write,
`Sec` or `Milli` where a number is expected, and `Nano` for data shared with `encoding/json` (v1).

## Common tasks

### Replacing the `format` tag

Since Go 1.27, `encoding/json/v2` rejects a struct that has a `format` tag option on a `time.Duration` field, so every
`json.Marshal` and `json.Unmarshal` of it fails. Remove the option, and choose the format in one of the two ways above:

```go
// Before Go 1.27
Timeout time.Duration `json:"timeout,format:iso8601"`

// With jsonduration.With(jsonduration.ISO8601) passed to json.Marshal and json.Unmarshal
Timeout time.Duration `json:"timeout"`

// Or without options
Timeout jsonduration.ISO8601Duration `json:"timeout"`
```

Stored data stays readable, since each format writes what the tag option of the same name wrote. A test checks this
against the tag option itself, whose code the standard library still contains and which go-json-experiment's
`ExperimentalSupportFormatTag` turns back on. Decoding differs only where jsonduration is stricter or exact:

- It reads `-0` as zero, where the tag option reports it as out of range.
- It rejects a fraction that is not all digits, such as `"PT1.e5H"`, which the tag option reads as one hour.
- It truncates fractions exactly, where the tag option rounds them through `float64`: in `Units` all of them, in
  `ISO8601` those of hours and minutes. So the last nanosecond can differ, as for `".9007199279999999s"` or
  `"PT0.99999999999999999999H"`, and at the very end of the range one may reject what the other accepts.
- The `string` tag option has no effect on `Units` and `ISO8601`, where the tag option rejects it.
- Its errors name their cause, such as `strconv.ErrSyntax` or `ErrCalendarUnits`.

### Migrating between formats

To write one format but also read others:

```go
// Stored data holds encoding/json (v1) nanoseconds; new data is ISO 8601.
opts := jsonduration.With(jsonduration.ISO8601, jsonduration.AlsoAccept(jsonduration.Nano))
```

Both `{"timeout":90000000000}` and `{"timeout":"PT1M30S"}` decode to 1m30s, and everything is written as ISO 8601. You
can accept several string formats, but only one numeric format, because a number such as `90` could be seconds or
nanoseconds.

### ISO 8601 days

`ISO8601` rejects days, weeks, months, and years, because a calendar day is not always 24 hours long. If the other side
means exactly 24 hours by a day, add `AcceptDays`. Python's Pydantic, and so FastAPI, writes a `timedelta` of 36 hours
as `"P1DT12H"`, and .NET's `XmlConvert` does the same for a `TimeSpan`:

```go
opts := jsonduration.With(jsonduration.ISO8601, jsonduration.AcceptDays())
// "P1DT12H" decodes to 36h
```

Months and years are still rejected, and output never uses days.

### Choosing the format at run time

`Format` works with `flag.TextVar` and in JSON configuration. Names must match the table exactly (`iso8601`, not
`ISO8601`), and anything else is an error that lists them:

```go
var format jsonduration.Format
flag.TextVar(&format, "duration-format", jsonduration.ISO8601, "how durations are written")
flag.Parse()
opts := jsonduration.With(format)
```

`With` panics on an invalid `Format`. A `Format` converted from a plain string, as in
`jsonduration.Format(os.Getenv("DURATION_FORMAT"))`, is not checked, so check it with `Valid()` first. The empty
`Format` is invalid too, and decoding leaves it for a missing name or `null`, so give it a default:

```go
opts := jsonduration.With(cmp.Or(cfg.DurationFormat, jsonduration.ISO8601))
```

### Without JSON

```go
jsonduration.ISO8601.Format(90 * time.Second) // "PT1M30S"
d, err := jsonduration.Sec.Parse("5400.5")    // 1h30m0.5s
```

## Good to know

- Decoding accepts only the formats you configured.
- To tell a bad duration in the input from other errors, as for a 400 Bad Request, check
  `errors.Is(err, jsonduration.ErrInvalidDuration)`. The `json.SemanticError` around it says where in the input it is.
- JSON `null` sets a duration to zero, and a `*time.Duration` to nil.
- Named types such as `type TTL time.Duration` are not `time.Duration`, so `With` does not affect them, and they are
  written as plain integers of nanoseconds.
- `ISO8601` takes a sign only in front, as in `-PT1H30M`, so it rejects negative durations as Java writes them, such as
  `PT-1H-30M`.
- To leave out zero durations, use the `omitzero` tag option. In v2, `omitempty` keeps them.
- If you pass your own `json.WithMarshalers`, combine it with `jsonduration.Marshalers` through `json.JoinMarshalers`,
  and likewise for unmarshalers. A second `WithMarshalers` option replaces the first one.
- If nanoseconds are all you need, an option of `encoding/json` is enough: import it as `jsonv1 "encoding/json"` and
  pass `jsonv1.FormatDurationAsNano(true)` to the v2 functions.

## Future

Go may add its own way to choose a format for a type, see [golang/go#71664](https://go.dev/issue/71664) and
[golang/go#81013](https://go.dev/issue/81013). If that happens, prefer it. A final release of this package will then
point to it.

## How it is tested

Examples alone can't show that a parser is right, so the tests also compare each codec with an independent
implementation of the same thing:

- **The removed `format` tag option.** The `internal/compat` module runs the tag option's own code, which the
  standard library still contains, next to jsonduration. Encoding must be byte-identical in every format, and decoding
  must agree except where [the list above](#replacing-the-format-tag) says otherwise.
- **Reference parsers.** A second parser for every format, built from regular expressions and `math/big` and sharing
  no code with the package, must agree on every input: whether it is accepted, its value, and the cause of an error.
- **`time.ParseDuration`.** `Units` must read what it reads, with the same value. Only fractions may differ, by the
  last nanosecond, because `time.ParseDuration` rounds them through `float64`.
- **Real producers.** Tests hold what Pydantic and .NET's `XmlConvert` write, which `AcceptDays` must read, and where
  `ISO8601` differs from Java's `Duration` on purpose.
- **Fuzzing.** Every comparison but the one with real producers is also a fuzz target. CI runs each target for 45
  seconds on every push and for 10 minutes every night, and keeps the inputs it finds between runs.
- **Mutation testing.** Every night, [gremlins](https://github.com/go-gremlins/gremlins) changes the code in small
  ways, such as `<` into `<=` or `+` into `-`, one change at a time. CI fails if too few of the changes make a test
  fail.
- **Coverage.** CI fails if any statement is not covered.

The tests run with the race detector, on Linux and macOS, on 32-bit, and with both the oldest supported and the latest
Go. `govulncheck` checks for known vulnerabilities on every push and every night.

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
included there.
