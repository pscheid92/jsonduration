# jsonduration

[![Go Reference](https://pkg.go.dev/badge/github.com/pscheid92/jsonduration.svg)](https://pkg.go.dev/github.com/pscheid92/jsonduration)
[![CI](https://github.com/pscheid92/jsonduration/actions/workflows/ci.yml/badge.svg)](https://github.com/pscheid92/jsonduration/actions/workflows/ci.yml)

JSON for `time.Duration` in Go's `encoding/json/v2`, in the format you choose.

`encoding/json/v2` has no default representation for `time.Duration`
([golang/go#71631](https://go.dev/issue/71631)): marshaling one fails with
an error. The `format` struct tag that used to pick one was removed in Go 1.27
([golang/go#79071](https://go.dev/issue/79071)). This package offers two
ways to choose a format instead:

- **One option per call** that covers every `time.Duration` in it, so your
  types stay as they are.
- **One wrapper type per format**, to choose per field without any options.

It is a small package for `encoding/json/v2` only. `encoding/json` (v1),
other JSON libraries, and YAML or TOML are out of scope.

```sh
go get github.com/pscheid92/jsonduration
```

Requires Go 1.27 or later. No dependencies outside the standard library.
Until v1.0.0 the API may still change.

## One format for a whole call

```go
import (
	"encoding/json/v2"
	"time"

	"github.com/pscheid92/jsonduration"
)

type Config struct {
	Timeout time.Duration   `json:"timeout"`
	Retry   []time.Duration `json:"retry"`
}

opts := jsonduration.With(jsonduration.ISO8601)

b, err := json.Marshal(Config{Timeout: 90 * time.Second}, opts)
// {"timeout":"PT1M30S","retry":[]}

var cfg Config
err = json.Unmarshal([]byte(`{"timeout":"PT2M"}`), &cfg, opts)
```

The format applies to struct fields, pointers, slices, arrays, map keys and
values. The options are safe for concurrent use, so create them once and
share them.

## One format per field

```go
type Config struct {
	Timeout jsonduration.ISO8601Duration `json:"timeout"`
	Backoff jsonduration.SecDuration     `json:"backoff"`
}

b, err := json.Marshal(Config{
	Timeout: jsonduration.ISO8601Duration{Duration: 90 * time.Second},
	Backoff: jsonduration.SecDuration{Duration: 1500 * time.Millisecond},
})
// {"timeout":"PT1M30S","backoff":1.5}

var cfg Config
err = json.Unmarshal(b, &cfg)
cfg.Timeout.Minutes()          // the methods of time.Duration still work
time.Sleep(cfg.Backoff.Duration)
```

Wrapper types embed `time.Duration` and implement `json.MarshalerTo` and
`json.UnmarshalerFrom`. `With` does not change their format.

## Formats

| Format      | Name        | Wrapper type        | JSON type | 1h30m0.5s       |
|-------------|-------------|---------------------|-----------|-----------------|
| `Units`     | `units`     | `UnitsDuration`     | string    | `"1h30m0.5s"`   |
| `Sec`       | `sec`       | `SecDuration`       | number    | `5400.5`        |
| `Milli`     | `milli`     | `MilliDuration`     | number    | `5400500`       |
| `Micro`     | `micro`     | `MicroDuration`     | number    | `5400500000`    |
| `Nano`      | `nano`      | `NanoDuration`      | number    | `5400500000000` |
| `ISO8601`   | `iso8601`   | `ISO8601Duration`   | string    | `"PT1H30M0.5S"` |
| `Base60`    | `base60`    | `Base60Duration`    | string    | `"1:30:00.5"`   |
| `ProtoJSON` | `protojson` | `ProtoJSONDuration` | string    | `"5400.500s"`   |

- The first six keep the names and the exact output of the formats
  `encoding/json/v2` offered through `format` before Go 1.27, so they can be
  carried over without changing any JSON.
- `Nano` is byte-for-byte what `encoding/json` (v1) writes.
- `Sec`, `Milli` and `Micro` are exact decimal numbers. They never go through
  `float64`.
- `ProtoJSON` is the protobuf JSON mapping of `google.protobuf.Duration`.
- Numeric formats become JSON strings where JSON needs one (object names)
  and when `json.StringifyNumbers` is set.

The codecs are also available without JSON:

```go
jsonduration.ProtoJSON.Format(1500 * time.Millisecond) // "1.500s"
d, err := jsonduration.Base60.Parse("1:30:00.5")         // 1h30m0.5s
```

A `Format` can be chosen at run time. It implements `MarshalText` and
`UnmarshalText`, so a format in a JSON configuration is checked while it is
decoded, and it works with `flag.TextVar`. The names in the table must match
exactly (`iso8601`, not `ISO8601`), and anything else is rejected with an
error that lists them:

```go
var format jsonduration.Format
flag.TextVar(&format, "duration-format", jsonduration.ISO8601, "how durations are written")
flag.Parse()
opts := jsonduration.With(format)
```

A `Format` converted from a plain string is not checked, and `With` panics
on an invalid one. Check it with `Valid` first:

```go
format := jsonduration.Format(os.Getenv("DURATION_FORMAT"))
if !format.Valid() {
	return fmt.Errorf("unknown duration format %q", format)
}
```

## Migrating between formats

Write one format, but read others too:

```go
// Stored data holds encoding/json (v1) nanoseconds; new data is ISO 8601.
opts := jsonduration.With(jsonduration.ISO8601, jsonduration.AlsoAccept(jsonduration.Nano))
```

`{"timeout":90000000000}` and `{"timeout":"PT1M30S"}` both decode to 1m30s,
and everything is written as ISO 8601. Old and new data can even be mixed in
one document.

At most one numeric format can be involved, because a number such as `90`
could be seconds or nanoseconds. `With` panics if you name two. String
formats never conflict: no text is valid in two of them with different
values. Where two accept the same text, such as `"1.5s"` in `Units` and
`ProtoJSON`, they agree on it. A fuzz test checks exactly this.

## Decoding is strict

- Only the configured formats are accepted. There is no guessing.
- `ISO8601` rejects years, months, weeks and days with `ErrNominalUnits`,
  since they have no fixed length.
- Where a producer means exactly 24 hours by a day, as Java's
  `Duration.parse` does, the `AcceptDays()` option to `With` reads
  `"P1DT12H"` as 36 hours and a week as 7 days. Years and months are still
  rejected, output never uses days, and `Parse` and the wrapper types stay
  strict.
- Digits below one nanosecond are truncated toward zero, computed exactly in
  every format. `Units` accepts exactly what `time.ParseDuration` accepts
  (a fuzz test compares the two), without going through `float64`.
- Errors wrap `strconv.ErrSyntax`, `strconv.ErrRange` or `ErrNominalUnits`.
- `null` sets a duration to zero, as `encoding/json/v2` does for other types.

## Good to know

- Named types such as `type TTL time.Duration` are different types and are
  not affected by `With`.
- A wrapper type reads and writes exactly its own format. To accept several
  formats on input, use `With` with `AlsoAccept`.
- Use `omitzero`, not `omitempty`, to leave out zero durations. In v2,
  `omitempty` only omits values that encode as empty JSON.
- If you pass your own `json.WithMarshalers`, combine it with
  `jsonduration.Marshalers` through `json.JoinMarshalers` (the same goes for
  unmarshalers). A second `WithMarshalers` option replaces the first one.
- The `string` tag option quotes numeric formats, like `json.StringifyNumbers`,
  and has no effect on string formats.

## Alternatives

- **The `json.FormatDurationAsNano` option** from `encoding/json`, passed to
  v2: nanoseconds only, as `encoding/json` (v1) writes them.
- **`github.com/go-json-experiment/json` with `ExperimentalSupportFormatTag`**:
  the old `format` struct tag, as an option that is explicitly temporary.
- **`tailscale.com/types/jsonformat`**: wrapper types for nanoseconds, Go
  strings and ISO 8601. It is part of the `tailscale.com` module and says it
  may be deleted once Go supports formats directly.
- **`github.com/MarkRosemaker/jsonutil`**: marshal functions for many types,
  among them durations as whole seconds or Go strings.

This package differs in offering both ways to choose (per call and per
field), all eight formats, exact decoding, and migration between formats,
with only the standard library as a dependency.

## Future

Go may add its own way to choose a format for a type: see
[golang/go#71664](https://go.dev/issue/71664) and
[golang/go#81013](https://go.dev/issue/81013), both still open proposals.
If that lands, prefer it. This package will then point to it in a final
release. The format names here are the ones Go used, so the move is meant to
be little more than a rename.

## Development

Needs [just](https://just.systems) and
[golangci-lint](https://golangci-lint.run).

```sh
just check         # gofmt, vet, golangci-lint, tests with -race, govulncheck
just fuzz          # every fuzz target for 30s; `just fuzz 2m FuzzParse` for one
just bench
just generate      # after changing gen_wrapper_types.go
```

The wrapper types in `wrapper_types.go` are generated from the table in
`gen_wrapper_types.go`. CI fails if the two are out of sync.

CI runs the same checks on Linux, macOS and Windows, fuzzes every push for a
short time and for longer once a week, and runs `govulncheck` weekly.
Renovate keeps the pinned GitHub Actions up to date.
Pushing a tag such as `v0.1.0` creates a GitHub release and announces the
version to the Go module proxy, so pkg.go.dev picks it up.

## License

BSD 3-Clause, see [LICENSE](LICENSE). Parts of the codecs are adapted from
the Go standard library, whose notice is included there.
