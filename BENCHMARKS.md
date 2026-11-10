# Benchmarks

What a duration costs with jsonduration, next to what the standard library takes for the same value without it.

- **It allocates nothing of its own.** Writing and reading a duration with `encoding/json/v2` allocates what an
  `int64` in the same place does: the JSON that `json.Marshal` returns, and nothing on reading. Only an error
  allocates.
- **`With` is cheap to call.** `With`, `Marshalers`, and `Unmarshalers` take 5 to 13 ns and allocate nothing, so there
  is no need to keep what they return.
- **A duration in a struct field takes about 50 ns more to write and 60 to 100 ns more to read than an `int64`.**
  About 35 ns of that is what the standard library takes to call any function for a type. The rest is the format.
- **Wrapper types are faster than options in a struct with many fields.** With options, the standard library asks for
  every field whether a function applies to it. With wrapper types it does not.

## How they were measured

- With Go 1.27.3 on a MacBook Pro with an Apple M2 and 16 GB of memory, macOS 27.0.1, `darwin/arm64`, in November
  2026.
- Every number is the median of ten runs of half a second each. `just bench` runs every benchmark once instead.

  ```sh
  go test -run '^$' -bench . -benchmem -benchtime 500ms -count 10
  ```

- Times are in nanoseconds for one operation. Two runs of the same benchmark differ by one or two percent, so a few
  nanoseconds between two rows mean nothing.
- The duration is 1h30m0.5s unless a section says otherwise: `"1h30m0.5s"`, `5400.5`, `5400500`, `5400500000`,
  `5400500000000`, and `"PT1H30M0.5S"` in the six formats.
- The benchmarks are in [bench_test.go](bench_test.go). Each heading below names its benchmark.

The tables compare jsonduration with four ways to do without it:

- **`int64`.** The field is an `int64` and there are no options. This is what a number costs in the standard library.
- **Nanosecond option.** A `time.Duration` with `jsonv1.FormatDurationAsNano(true)`, the standard library's own way to
  write a duration, as nanoseconds only.
- **Smallest function.** A `time.Duration` with `json.WithMarshalers` and `json.WithUnmarshalers`, and functions that
  do nothing but write and read the nanoseconds as an integer. `With` passes its functions the same way, so this is
  the least that it could cost.
- **Text methods.** A type of your own with `MarshalText` and `UnmarshalText`, which call `time.Duration.String` and
  `time.ParseDuration`. This is what one writes by hand.

## Building the options

`BenchmarkOptions`

| Call                                                    | Time | Allocations |
|---------------------------------------------------------|-----:|------------:|
| `With(ISO8601)`                                         |  5.5 |           0 |
| `With(ISO8601, AlsoAccept(Nano, Units), AcceptDays())`  | 12.8 |           0 |
| `Marshalers(ISO8601)`                                   |  5.1 |           0 |
| `Unmarshalers(ISO8601, AlsoAccept(Nano))`               |  8.7 |           0 |

The options are built once for each set of arguments and shared afterwards. Versions up to v0.3.1 built them on
every call, which took about 260 ns and 13 allocations for `With(ISO8601)`, so keep the options in a variable there.

## Writing one duration

`BenchmarkMarshal`: `json.Marshal` of a pointer to a `struct{ D time.Duration }`.

| Format    | Options kept | `With` on every call | Wrapper type |
|-----------|-------------:|---------------------:|-------------:|
| `Units`   |          152 |                  159 |          148 |
| `Sec`     |          156 |                  161 |          146 |
| `Milli`   |          152 |                  159 |          141 |
| `Micro`   |          155 |                  160 |          139 |
| `Nano`    |          157 |                  162 |          143 |
| `ISO8601` |          157 |                  161 |          141 |

Each of these allocates once, 16 or 24 bytes: the JSON that `json.Marshal` returns.

| Without jsonduration | Time | Allocations |
|----------------------|-----:|------------:|
| `int64`              |  102 |           1 |
| Nanosecond option    |  104 |           1 |
| Smallest function    |  136 |           1 |
| Text methods         |  151 |           2 |

`BenchmarkMarshalWrite` writes into an `io.Writer` with `json.MarshalWrite`, which returns no JSON. Then nothing is
left to allocate:

| Way                      |       Time | Allocations |
|--------------------------|-----------:|------------:|
| jsonduration, any format | 145 to 162 |           0 |
| `int64`                  |         94 |           0 |
| Nanosecond option        |         97 |           0 |
| Smallest function        |        131 |           0 |
| Text methods             |        154 |           1 |

## Reading one duration

`BenchmarkUnmarshal`: `json.Unmarshal` into the same struct.

| Format    | Options kept | `With` on every call | Wrapper type |
|-----------|-------------:|---------------------:|-------------:|
| `Units`   |          207 |                  208 |          182 |
| `Sec`     |          180 |                  187 |          153 |
| `Milli`   |          174 |                  174 |          145 |
| `Micro`   |          177 |                  178 |          149 |
| `Nano`    |          180 |                  181 |          153 |
| `ISO8601` |          212 |                  213 |          183 |

None of these allocates.

| Without jsonduration | Time | Allocations |
|----------------------|-----:|------------:|
| `int64`              |  115 |           0 |
| Nanosecond option    |  116 |           0 |
| Smallest function    |  152 |           0 |
| Text methods         |  174 |           1 |

## Reading more than one format

`BenchmarkUnmarshalAlsoAccept`

| Options                             | Input             | Time | Allocations |
|-------------------------------------|-------------------|-----:|------------:|
| `With(ISO8601)`                     | `"PT1H30M0.5S"`   |  211 |           0 |
| `With(ISO8601, AlsoAccept(Units))`  | `"PT1H30M0.5S"`   |  212 |           0 |
| `With(ISO8601, AlsoAccept(Units))`  | `"1h30m0.5s"`     |  213 |           0 |
| `With(ISO8601, AlsoAccept(Nano))`   | `5400500000000`   |  181 |           0 |

A second format costs nothing that can be measured, whether the input is in the first format or in the second.

## Many durations

`BenchmarkSlice`: a `[]time.Duration` of a thousand durations, from 1h30m0.5s on in steps of one second. The times
are for one duration, a thousandth of the whole.

| Way               | Writing | Reading |
|-------------------|--------:|--------:|
| `Units`           |    46.6 |   101.0 |
| `Sec`             |    47.6 |    65.1 |
| `Milli`           |    41.6 |    59.1 |
| `Micro`           |    43.4 |    64.2 |
| `Nano`            |    45.1 |    68.7 |
| `ISO8601`         |    47.0 |   106.4 |
| `int64`           |    14.8 |    33.2 |
| Nanosecond option |    16.3 |    34.4 |
| Smallest function |    27.2 |    41.9 |

Writing the thousand allocates once, for the JSON, and reading into a slice that is long enough allocates nothing, in
every row. The number of allocations does not grow with the number of durations.

## A struct with other fields

`BenchmarkConfig`: nine values, five of them durations.

```go
type config struct {
	Name     string
	Address  string
	Retries  int
	Verbose  bool
	Timeout  time.Duration
	Interval time.Duration
	Backoff  []time.Duration // three of them
}
```

| Way               | Writing, options | Writing, wrapper types | Reading, options | Reading, wrapper types |
|-------------------|-----------------:|-----------------------:|-----------------:|-----------------------:|
| `Units`           |              605 |                    533 |              825 |                    729 |
| `Sec`             |              609 |                    521 |              818 |                    697 |
| `Milli`           |              616 |                    534 |              816 |                    687 |
| `Micro`           |              618 |                    533 |              819 |                    709 |
| `Nano`            |              632 |                    548 |              837 |                    733 |
| `ISO8601`         |              581 |                    495 |              908 |                    800 |
| `int64`           |              346 |                        |              535 |                        |
| Nanosecond option |              350 |                        |              536 |                        |
| Smallest function |              527 |                        |              681 |                        |

Writing allocates once, for the JSON, and reading allocates nothing, in every row.

The smallest function takes 180 ns more than `int64` to write this struct, but only 35 ns more to write the struct
with one field. That is because the standard library, once it has functions from `json.WithMarshalers` or
`json.WithUnmarshalers`, asks for every field whether one of them applies to its type, each time it writes or reads
the struct. The strings, the integer, and the boolean pay for it too. A wrapper type needs no options, so nothing is
asked, and that is why wrapper types are 70 to 130 ns faster here.

## Invalid input

`BenchmarkUnmarshalInvalid`: `{"D":"1h30m"}`, which is rejected. The error builds its message when `Error` is
called, so the table has the cost for a caller who only checks the error, with `errors.Is` for example, and for one
who also reads the message.

| Way                               | Checked: time | Bytes | Allocations | Read: time | Bytes | Allocations |
|-----------------------------------|--------------:|------:|------------:|-----------:|------:|------------:|
| `With(ISO8601)`                   |           292 |   192 |           5 |        516 |   528 |          13 |
| `int64`, which rejects the string |           169 |   128 |           3 |        273 |   256 |           8 |

An error is the one thing that allocates. Until its message is read, a syntax error like this one takes two
allocations more than the standard library's own error: the input that it quotes, and the error itself. The message
says where the duration is, quotes the input, and names the accepted formats and the cause, which the standard
library's message for an `int64` does not.

## Without JSON

`BenchmarkFormat` and `BenchmarkParse`: `Format.Format` and `Format.Parse`, next to the two functions of package
`time`.

| Format                                       | `Format` | Bytes | Allocations | `Parse` | Allocations |
|----------------------------------------------|---------:|------:|------------:|--------:|------------:|
| `Units`                                      |     29.9 |    16 |           1 |    59.9 |           0 |
| `Sec`                                        |     19.6 |     8 |           1 |    20.5 |           0 |
| `Milli`                                      |     19.4 |     8 |           1 |    20.8 |           0 |
| `Micro`                                      |     21.6 |    16 |           1 |    22.1 |           0 |
| `Nano`                                       |     22.8 |    16 |           1 |    24.1 |           0 |
| `ISO8601`                                    |     31.5 |    16 |           1 |    58.7 |           0 |
| `time.Duration.String`, `time.ParseDuration` |     22.7 |    16 |           1 |    45.0 |           0 |

`Format` allocates the string that it returns. `Parse` allocates nothing for input of up to 32 bytes and once for
longer input. Nothing that `Format` writes is longer than that.
