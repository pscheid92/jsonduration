# Security policy

## Supported versions

Only the latest release receives fixes. Until v1.0.0, a fix may come with changes to the API.

## Reporting a vulnerability

Please do not open a public issue for security problems. Report them privately instead:
<https://github.com/pscheid92/jsonduration/security/advisories/new>.

Include the jsonduration and Go versions, the format and options, and the JSON that triggers the problem. You will get
an acknowledgement within a few days, and a fix or a workaround as soon as one is available.

## What counts

jsonduration decodes durations from JSON that may come from anyone, such as the body of an HTTP request. Report it if
decoding any input:

- panics,
- takes time or memory out of proportion to the length of the input,
- or returns a duration other than the one the input means, instead of an error.

A value that is valid but large, such as `"2562047h"`, is not a vulnerability. If a duration of that size is a risk in
your program, as a timeout or a sleep, check its range after decoding.

## Design

The package has no dependencies outside the standard library and does not use `unsafe`. The codecs use no regular
expressions and compute in integers, without floating-point. The tests check them against independent implementations,
also with fuzzing; see [How it is tested](README.md#how-it-is-tested).
