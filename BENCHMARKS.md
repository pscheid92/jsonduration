# Benchmarks

## What the tests enforce

These hold on every platform and Go version that CI runs, because a test fails otherwise:

- **Writing and reading a duration allocates nothing of its own.** What is allocated is what the standard library
  allocates for an `int64` in the same place: the JSON that `json.Marshal` returns, and nothing on reading.
- **`With`, `Marshalers`, and `Unmarshalers` allocate nothing without decoding options.** The options of each format
  are built once, so there is no need to keep what these return. With `AlsoAccept` or `AcceptDays` they build new
  options on every call, so keep those in a variable. Versions up to v0.3.1 built every set of options anew.
- **`Format.Format` allocates the string it returns, and `Format.Parse` nothing** for what `Format.Format` writes.

Only an error allocates. It builds its message when `Error` is called, so a caller who only checks it with
`errors.Is` does not pay for the message.

## Roughly how long it takes

One duration of 1h30m0.5s in a `struct{ D time.Duration }`, in nanoseconds, with Go 1.27.3 on an Apple M2 in November
2026. This is one measurement on one machine and will drift, so take the proportions from it and not the numbers.

| Way                                                            |    Writing |    Reading |
|----------------------------------------------------------------|-----------:|-----------:|
| An `int64` field, without options                              |        102 |        115 |
| A function that does nothing but write or read the nanoseconds |        136 |        152 |
| A type of your own with `MarshalText` and `UnmarshalText`      |        151 |        174 |
| `With`, in the six formats                                     | 152 to 157 | 174 to 212 |
| A wrapper type such as `ISO8601Duration`, in the six formats   | 139 to 148 | 145 to 183 |

- **A large part of the difference to an `int64` is the standard library's.** The second row passes its function the
  way `With` does, with `json.WithMarshalers` and `json.WithUnmarshalers`, so it is the least that `With` could cost.
- **A type of your own is no faster.** It also allocates once more than the other rows, each way.
- **Wrapper types are faster than options in a struct with many fields.** Once it has functions, the standard library
  asks for every field whether one of them applies to its type, so the fields that are not durations pay too. A
  wrapper type needs no options.
- **A second format costs nothing that can be measured.** Reading with `AlsoAccept` takes as long as without, whether
  the input is in the first format or in the second.

## Measuring it yourself

`just bench` runs the benchmarks in [bench_test.go](bench_test.go): writing and reading one duration, `AlsoAccept`,
a struct with other fields, and `Format` and `Parse`, each next to the ways above to do without the package.
