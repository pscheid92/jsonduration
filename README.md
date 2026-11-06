# jsonduration

[![CI](https://github.com/pscheid92/jsonduration/actions/workflows/ci.yml/badge.svg)](https://github.com/pscheid92/jsonduration/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/pscheid92/jsonduration.svg)](https://pkg.go.dev/github.com/pscheid92/jsonduration)
[![Coverage](https://img.shields.io/endpoint?url=https%3A%2F%2Fraw.githubusercontent.com%2Fpscheid92%2Fjsonduration%2Fbadges%2Fcoverage.json)](https://github.com/pscheid92/jsonduration/actions/workflows/ci.yml)

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

The format applies to fields, pointers, slices, arrays, and map keys and values. Create the options once and share them:
they are safe for concurrent use.

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

## Formats

| Format      | Name        | Wrapper type        | JSON type | 1h30m0.5s       |
|-------------|-------------|---------------------|-----------|-----------------|
| `Units`     | `units`     | `UnitsDuration`     | string    | `"1h30m0.5s"`   |
| `Sec`       | `sec`       | `SecDuration`       | number    | `5400.5`        |
| `Milli`     | `milli`     | `MilliDuration`     | number    | `5400500`       |
| `Micro`     | `micro`     | `MicroDuration`     | number    | `5400500000`    |
| `Nano`      | `nano`      | `NanoDuration`      | number    | `5400500000000` |
| `ISO8601`   | `iso8601`   | `ISO8601Duration`   | string    | `"PT1H30M0.5S"` |

- These are the six formats the `format` tag offered before Go 1.27, and they write the same JSON as they did then.
- `Nano` writes what `encoding/json` (v1) writes.
- Numeric formats are not rounded.

## Common tasks

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

`ISO8601` rejects days, weeks, months and years, because a calendar day is not always 24 hours long. If the other side
means exactly 24 hours by a day, as Java's `Duration.parse` does, add `AcceptDays`:

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

A `Format` converted from a plain string, as in `jsonduration.Format(os.Getenv("DURATION_FORMAT"))`, is not checked, and
`With` panics on an invalid one. Check it with `Valid()` first.

### Without JSON

```go
jsonduration.ISO8601.Format(90 * time.Second) // "PT1M30S"
d, err := jsonduration.Sec.Parse("5400.5")    // 1h30m0.5s
```

## Good to know

- Decoding accepts only the formats you configured.
- Named types such as `type TTL time.Duration` are not `time.Duration`, so `With` does not affect them.
- To leave out zero durations, use the `omitzero` tag option. In v2, `omitempty` keeps them.
- If you pass your own `json.WithMarshalers`, combine it with `jsonduration.Marshalers` through `json.JoinMarshalers`,
  and likewise for unmarshalers. A second `WithMarshalers` option replaces the first one.
- If nanoseconds are all you need, the `FormatDurationAsNano(true)` option from `encoding/json`, passed to the v2
  functions, is enough.

## Future

Go may add its own way to choose a format for a type, see [golang/go#71664](https://go.dev/issue/71664)
and [golang/go#81013](https://go.dev/issue/81013). If that happens, prefer it. A final release of this package will then
point to it.

## Development

Needs [just](https://just.systems) and [golangci-lint](https://golangci-lint.run). Run `just` to list all recipes.

```sh
just check      # lint, tests with -race, govulncheck
just fuzz       # every fuzz target for 30s
just generate   # after changing gen_wrapper_types.go
```

## License

BSD 3-Clause, see [LICENSE](LICENSE). Parts of the codecs are adapted from the Go standard library, whose notice is
included there.
