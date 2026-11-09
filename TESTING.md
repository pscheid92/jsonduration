# How it is tested

Examples alone can't show that a parser is right, so the tests also compare each codec with an independent
implementation of the same thing:

- **The removed `format` tag option.** The `internal/compat` module runs the tag option's own code, which the
  standard library still contains, next to jsonduration. Encoding must be byte-identical in every format, and decoding
  must agree except where [the README](README.md#replacing-the-format-tag) says otherwise.
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
