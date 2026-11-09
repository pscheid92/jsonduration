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

Pass one option per call. It covers every `time.Duration` in the call: fields, pointers, slices, arrays, and map keys
and values. The options are safe for concurrent use, so create them once.

```go
type Config struct {
	Timeout time.Duration   `json:"timeout"`
	Retries []time.Duration `json:"retries"`
}

opts := jsonduration.With(jsonduration.ISO8601)

b, err := json.Marshal(cfg, opts) // {"timeout":"PT1M30S","retries":["PT1S","PT5S"]}
err = json.Unmarshal(b, &cfg, opts)
```

Or use one wrapper type per field, if you can't pass options or if fields need different formats:

```go
type Config struct {
	Timeout jsonduration.ISO8601Duration `json:"timeout"` // "PT1M30S"
	Backoff jsonduration.SecDuration     `json:"backoff"` // 1.5
}
```

A wrapper type embeds `time.Duration`, so `cfg.Timeout.Minutes()` keeps working. The
[package documentation](https://pkg.go.dev/github.com/pscheid92/jsonduration) has runnable examples of both.

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

Numeric formats are exact. But a decoder that reads JSON numbers as `float64`, such as JavaScript's `JSON.parse`, can
lose the nanoseconds of long durations.

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

### ISO 8601 days

`ISO8601` rejects days, weeks, months, and years, because a calendar day is not always 24 hours long. If the other side
means exactly 24 hours by a day, as Python's Pydantic and .NET's `XmlConvert` do, add `AcceptDays`:

```go
opts := jsonduration.With(jsonduration.ISO8601, jsonduration.AcceptDays())
// "P1DT12H" decodes to 36h
```

Months and years stay rejected, and output never uses days. A count of zero, as in `"P0D"` or `"P0Y0M0DT1H30M0S"`, is
always read.

### Options for a wrapper type

The wrapper types take no options. If a field needs `AcceptDays` or `AlsoAccept`, write your own:

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

### Choosing the format at run time

`Format` works with `flag.TextVar` and in JSON configuration. Names must match the table exactly:

```go
var format jsonduration.Format
flag.TextVar(&format, "duration-format", jsonduration.ISO8601, "how durations are written")
flag.Parse()
opts := jsonduration.With(format)
```

`With` panics on an invalid `Format`, which includes the empty one. Check a `Format` converted from a plain string
with `Valid()` first.

### Without JSON

```go
jsonduration.ISO8601.Format(90 * time.Second) // "PT1M30S"
d, err := jsonduration.Sec.Parse("5400.5")    // 1h30m0.5s
```

## Good to know

- Every error about a bad duration matches `jsonduration.ErrInvalidDuration`, and its cause: `strconv.ErrSyntax`,
  `strconv.ErrRange`, `ErrCalendarUnits`, or `ErrWrongType`. The `json.SemanticError` around it says where it is.
- Named types such as `type TTL time.Duration` are not `time.Duration`, so `With` does not affect them.
- JSON `null` sets a duration to zero, and a `*time.Duration` to nil.
- To leave out zero durations, use `omitzero`. In v2, `omitempty` keeps them.
- If you pass your own `json.WithMarshalers`, combine it with `jsonduration.Marshalers` through
  `json.JoinMarshalers`, and likewise for unmarshalers. A second `WithMarshalers` option replaces the first one.
- `ISO8601` takes a sign only in front, as in `-PT1H30M`. It rejects `PT-1H-30M`, as Java writes it.
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
