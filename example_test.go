package jsonduration_test

import (
	"cmp"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/pscheid92/jsonduration"
)

func Example() {
	type Config struct {
		Timeout time.Duration   `json:"timeout"`
		Retry   []time.Duration `json:"retry"`
		Grace   *time.Duration  `json:"grace,omitzero"`
	}

	opts := jsonduration.With(jsonduration.ISO8601)

	cfg := Config{
		Timeout: 90 * time.Second,
		Retry:   []time.Duration{time.Second, 2500 * time.Millisecond},
	}

	b, err := json.Marshal(cfg, opts)
	fmt.Println(string(b), err)

	var c Config
	err = json.Unmarshal([]byte(`{"timeout":"PT2M","retry":["PT0.5S"]}`), &c, opts)
	fmt.Println(c.Timeout, c.Retry, err)

	// Output:
	// {"timeout":"PT1M30S","retry":["PT1S","PT2.5S"]} <nil>
	// 2m0s [500ms] <nil>
}

// Wrapper types choose the format per field and need no options. The methods of time.Duration stay available.
func ExampleISO8601Duration() {
	type Config struct {
		Timeout jsonduration.ISO8601Duration `json:"timeout"`
		Backoff jsonduration.SecDuration     `json:"backoff"`
	}

	cfg := Config{
		Timeout: jsonduration.ISO8601Duration{Duration: 90 * time.Second},
		Backoff: jsonduration.SecDuration{Duration: 1500 * time.Millisecond},
	}

	b, err := json.Marshal(cfg)
	fmt.Println(string(b), err)

	var c Config
	err = json.Unmarshal([]byte(`{"timeout":"PT2M","backoff":0.25}`), &c)
	fmt.Println(c.Timeout.Minutes(), c.Backoff, err)

	// Output:
	// {"timeout":"PT1M30S","backoff":1.5} <nil>
	// 2 250ms <nil>
}

// Read data that encoding/json (v1) wrote as nanoseconds, while writing ISO 8601 from now on.
func ExampleAlsoAccept() {
	type Job struct {
		Timeout time.Duration `json:"timeout"`
	}

	opts := jsonduration.With(jsonduration.ISO8601, jsonduration.AlsoAccept(jsonduration.Nano))

	inputs := []string{`{"timeout":90000000000}`, `{"timeout":"PT1M30S"}`}

	for _, stored := range inputs {
		var j Job
		err := json.Unmarshal([]byte(stored), &j, opts)
		fmt.Println(j.Timeout, err)
	}

	job := Job{Timeout: 90 * time.Second}
	b, err := json.Marshal(job, opts)
	fmt.Println(string(b), err)

	// Output:
	// 1m30s <nil>
	// 1m30s <nil>
	// {"timeout":"PT1M30S"} <nil>
}

// Read what Python's Pydantic writes for a timedelta of 36 hours, where a day is exactly 24 hours.
func ExampleAcceptDays() {
	opts := jsonduration.With(jsonduration.ISO8601, jsonduration.AcceptDays())

	var d time.Duration
	err := json.Unmarshal([]byte(`"P1DT12H"`), &d, opts)
	fmt.Println(d, err)

	// Months still have no fixed length.
	err = json.Unmarshal([]byte(`"P1M"`), &d, opts)
	fmt.Println(errors.Is(err, jsonduration.ErrCalendarUnits))

	// Output:
	// 36h0m0s <nil>
	// true
}

// Tell a bad duration in a request from other errors, for example to answer with 400 Bad Request,
// and find where in the request it is.
func Example_invalidDuration() {
	type Request struct {
		Timeout time.Duration `json:"timeout"`
	}

	opts := jsonduration.With(jsonduration.ISO8601)
	body := []byte(`{"timeout":"90s"}`)

	var req Request
	err := json.Unmarshal(body, &req, opts)
	fmt.Println(errors.Is(err, jsonduration.ErrInvalidDuration))

	// The json.SemanticError around the error says where the duration is.
	// Its own message varies on purpose, so print only the parts.
	if semantic, ok := errors.AsType[*json.SemanticError](err); ok {
		fmt.Println(semantic.JSONPointer)
		fmt.Println(semantic.Err)
	}

	// Output:
	// true
	// /timeout
	// invalid duration "90s" in format iso8601: invalid syntax
}

// Tell a JSON value of the wrong type, such as a number where the format writes a string, from a string that is
// not a duration. Such an error matches ErrInvalidDuration as well.
func Example_wrongType() {
	opts := jsonduration.With(jsonduration.ISO8601)

	var d time.Duration
	err := json.Unmarshal([]byte(`90`), &d, opts)
	fmt.Println(errors.Is(err, jsonduration.ErrWrongType))
	fmt.Println(errors.Is(err, jsonduration.ErrInvalidDuration))

	if semantic, ok := errors.AsType[*json.SemanticError](err); ok {
		fmt.Println(semantic.Err)
	}

	// Output:
	// true
	// true
	// invalid duration "90" in format iso8601: want a JSON string
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
}

func ExampleFormat_Parse() {
	d, err := jsonduration.ISO8601.Parse("PT1H30M")
	fmt.Println(d, err)

	// Days have no fixed length.
	_, err = jsonduration.ISO8601.Parse("P1D")
	fmt.Println(errors.Is(err, jsonduration.ErrCalendarUnits))

	// Only the format itself is accepted.
	_, err = jsonduration.ISO8601.Parse("1h30m")
	fmt.Println(err)

	// Output:
	// 1h30m0s <nil>
	// true
	// invalid duration "1h30m" in format iso8601: invalid syntax
}

// Choose the format at run time, from a flag. A format in a JSON configuration works the same way.
func ExampleFormat_UnmarshalText() {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var format jsonduration.Format
	fs.TextVar(&format, "duration-format", jsonduration.ISO8601, "how durations are written")

	args := []string{"-duration-format=sec"}
	_ = fs.Parse(args)

	opts := jsonduration.With(format)
	timeout := 90 * time.Second

	b, err := json.Marshal(timeout, opts)
	fmt.Println(string(b), err)

	// Unknown names are rejected.
	args = []string{"-duration-format=weeks"}
	err = fs.Parse(args)
	fmt.Println(err != nil)

	// Output:
	// 90 <nil>
	// true
}

// Read the format from a JSON configuration.
// A missing name or null leaves the Format empty, which is not valid, so give it a default with cmp.Or.
func ExampleFormat_UnmarshalText_configuration() {
	type Settings struct {
		DurationFormat jsonduration.Format `json:"durationFormat"`
	}

	inputs := []string{`{"durationFormat":"sec"}`, `{}`, `{"durationFormat":null}`}

	for _, in := range inputs {
		var settings Settings
		err := json.Unmarshal([]byte(in), &settings)

		format := cmp.Or(settings.DurationFormat, jsonduration.ISO8601)
		fmt.Println(format, err)
	}

	// Output:
	// sec <nil>
	// iso8601 <nil>
	// iso8601 <nil>
}

// Numeric formats are written as JSON strings in object names, as JSON requires, and with json.StringifyNumbers.
func ExampleWith_numbersAsStrings() {
	m := map[time.Duration]time.Duration{time.Minute: time.Hour}

	opts := jsonduration.With(jsonduration.Sec)
	stringify := json.StringifyNumbers(true)

	b, _ := json.Marshal(m, opts)
	fmt.Println(string(b))

	b, _ = json.Marshal(m, opts, stringify)
	fmt.Println(string(b))

	// Output:
	// {"60":3600}
	// {"60":"3600"}
}

// Combine the duration marshaler with your own through json.JoinMarshalers,
// because a second json.WithMarshalers option would replace the first.
func ExampleMarshalers() {
	type UserID int

	type Session struct {
		ID  UserID
		TTL time.Duration
	}

	writeUserID := func(enc *jsontext.Encoder, id UserID) error {
		name := fmt.Sprintf("user-%d", id)
		token := jsontext.String(name)
		return enc.WriteToken(token)
	}

	ids := json.MarshalToFunc(writeUserID)
	durations := jsonduration.Marshalers(jsonduration.Units)
	opts := json.WithMarshalers(json.JoinMarshalers(ids, durations))

	session := Session{ID: 42, TTL: 90 * time.Minute}

	b, err := json.Marshal(session, opts)
	fmt.Println(string(b), err)

	// Output:
	// {"ID":"user-42","TTL":"1h30m0s"} <nil>
}
