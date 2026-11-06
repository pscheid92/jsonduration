// Copyright 2026 Patrick Scheid. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonduration_test

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/pscheid92/jsonduration"
)

// Read days from a service that means 24 hours by one, as Java's
// Duration.parse does.
func ExampleAcceptDays() {
	opts := jsonduration.With(jsonduration.ISO8601, jsonduration.AcceptDays())

	var d time.Duration
	err := json.Unmarshal([]byte(`"P1DT12H"`), &d, opts)
	fmt.Println(d, err)

	// Years and months still have no fixed length.
	err = json.Unmarshal([]byte(`"P1M"`), &d, opts)
	fmt.Println(errors.Is(err, jsonduration.ErrNominalUnits))

	// Output:
	// 36h0m0s <nil>
	// true
}

// Choose the format at run time, from a flag. The same works for a format
// in a JSON configuration. Names must match exactly, and anything else is
// rejected right away.
func ExampleFormat_UnmarshalText() {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // keep the usage message out of the example
	var format jsonduration.Format
	fs.TextVar(&format, "duration-format", jsonduration.ISO8601, "how durations are written")

	_ = fs.Parse([]string{"-duration-format=sec"})
	b, err := json.Marshal(90*time.Second, jsonduration.With(format))
	fmt.Println(string(b), err)

	fmt.Println(fs.Parse([]string{"-duration-format=weeks"}) != nil)

	// Output:
	// 90 <nil>
	// true
}

func Example() {
	type Config struct {
		Timeout time.Duration   `json:"timeout"`
		Retry   []time.Duration `json:"retry"`
		Grace   *time.Duration  `json:"grace,omitzero"`
	}
	opts := jsonduration.With(jsonduration.ISO8601)

	b, err := json.Marshal(Config{
		Timeout: 90 * time.Second,
		Retry:   []time.Duration{time.Second, 2500 * time.Millisecond},
	}, opts)
	fmt.Println(string(b), err)

	var c Config
	err = json.Unmarshal([]byte(`{"timeout":"PT2M","retry":["PT0.5S"]}`), &c, opts)
	fmt.Println(c.Timeout, c.Retry, err)

	// Output:
	// {"timeout":"PT1M30S","retry":["PT1S","PT2.5S"]} <nil>
	// 2m0s [500ms] <nil>
}

func ExampleFormat_Format() {
	d := 90*time.Minute + 500*time.Millisecond
	for _, f := range jsonduration.Formats() {
		fmt.Printf("%-9s %s\n", f, f.Format(d))
	}
	// Output:
	// units     1h30m0.5s
	// sec       5400.5
	// milli     5400500
	// micro     5400500000
	// nano      5400500000000
	// iso8601   PT1H30M0.5S
	// base60    1:30:00.5
	// protojson 5400.500s
}

func ExampleFormat_Parse() {
	d, err := jsonduration.ISO8601.Parse("PT1H30M")
	fmt.Println(d, err)

	// Days have no fixed length, so they are rejected.
	_, err = jsonduration.ISO8601.Parse("P1D")
	fmt.Println(errors.Is(err, jsonduration.ErrNominalUnits))

	// Only the configured format is accepted.
	_, err = jsonduration.ISO8601.Parse("1h30m")
	fmt.Println(err)

	// Output:
	// 1h30m0s <nil>
	// true
	// invalid ISO 8601 duration "1h30m": invalid syntax
}

// Combine the duration marshaler with your own through json.JoinMarshalers,
// because a second json.WithMarshalers option would replace the first.
func ExampleMarshalers() {
	type UserID int
	ids := json.MarshalToFunc(func(enc *jsontext.Encoder, id UserID) error {
		return enc.WriteToken(jsontext.String(fmt.Sprintf("user-%d", id)))
	})
	opts := json.WithMarshalers(json.JoinMarshalers(ids, jsonduration.Marshalers(jsonduration.Units)))

	b, err := json.Marshal(struct {
		ID  UserID
		TTL time.Duration
	}{42, 90 * time.Minute}, opts)
	fmt.Println(string(b), err)

	// Output:
	// {"ID":"user-42","TTL":"1h30m0s"} <nil>
}

// Numeric formats are written as JSON strings where JSON requires one,
// such as object names, and wherever json.StringifyNumbers asks for it.
func ExampleFormat_IsNumeric() {
	m := map[time.Duration]time.Duration{time.Minute: time.Hour}

	b, _ := json.Marshal(m, jsonduration.With(jsonduration.Sec))
	fmt.Println(string(b))

	b, _ = json.Marshal(m, jsonduration.With(jsonduration.Sec), json.StringifyNumbers(true))
	fmt.Println(string(b))

	// Output:
	// {"60":3600}
	// {"60":"3600"}
}

// Read data written by encoding/json (v1), which stores nanoseconds, while
// writing ISO 8601 from now on.
func ExampleAlsoAccept() {
	type Job struct {
		Timeout time.Duration `json:"timeout"`
	}
	opts := jsonduration.With(jsonduration.ISO8601, jsonduration.AlsoAccept(jsonduration.Nano))

	for _, stored := range []string{`{"timeout":90000000000}`, `{"timeout":"PT1M30S"}`} {
		var j Job
		err := json.Unmarshal([]byte(stored), &j, opts)
		fmt.Println(j.Timeout, err)
	}

	b, err := json.Marshal(Job{Timeout: 90 * time.Second}, opts)
	fmt.Println(string(b), err)

	// Output:
	// 1m30s <nil>
	// 1m30s <nil>
	// {"timeout":"PT1M30S"} <nil>
}

// Wrapper types choose the format per field and need no options. The
// methods of time.Duration stay available.
func ExampleISO8601Duration() {
	type Config struct {
		Timeout jsonduration.ISO8601Duration `json:"timeout"`
		Backoff jsonduration.SecDuration     `json:"backoff"`
	}
	b, err := json.Marshal(Config{
		Timeout: jsonduration.ISO8601Duration{Duration: 90 * time.Second},
		Backoff: jsonduration.SecDuration{Duration: 1500 * time.Millisecond},
	})
	fmt.Println(string(b), err)

	var c Config
	err = json.Unmarshal([]byte(`{"timeout":"PT2M","backoff":0.25}`), &c)
	fmt.Println(c.Timeout.Minutes(), c.Backoff, err)

	// Output:
	// {"timeout":"PT1M30S","backoff":1.5} <nil>
	// 2 250ms <nil>
}
