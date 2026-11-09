package jsonduration_test

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"time"

	"github.com/pscheid92/jsonduration"
)

var unitsOptions = jsonduration.With(jsonduration.Units)

// TTL is a named type. [jsonduration.With] does not affect it, since it is not a time.Duration, so it encodes itself.
type TTL time.Duration

func (t TTL) MarshalJSONTo(enc *jsontext.Encoder) error {
	return json.MarshalEncode(enc, time.Duration(t), unitsOptions)
}

func (t *TTL) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return json.UnmarshalDecode(dec, (*time.Duration)(t), unitsOptions)
}

// A named type such as TTL takes two short methods to get a format.
func ExampleWith_namedType() {
	type Entry struct {
		Key string `json:"key"`
		TTL TTL    `json:"ttl"`
	}

	b, err := json.Marshal(Entry{Key: "k", TTL: TTL(5 * time.Minute)})
	fmt.Println(string(b), err)

	var e Entry
	err = json.Unmarshal([]byte(`{"key":"k","ttl":"90s"}`), &e)
	fmt.Println(time.Duration(e.TTL), err)

	// Output:
	// {"key":"k","ttl":"5m0s"} <nil>
	// 1m30s <nil>
}

var secOptions = jsonduration.With(jsonduration.Sec)

// RetryPolicy belongs to a package that others encode without options. Its fields stay time.Duration, so the
// package's users can still write InitialBackoff: 250 * time.Millisecond, and the struct encodes itself.
type RetryPolicy struct {
	MaxAttempts    int           `json:"max_attempts"`
	InitialBackoff time.Duration `json:"initial_backoff"`
}

// plainRetryPolicy has the fields of RetryPolicy but not its methods, which would otherwise call themselves.
type plainRetryPolicy RetryPolicy

func (p RetryPolicy) MarshalJSONTo(enc *jsontext.Encoder) error {
	return json.MarshalEncode(enc, plainRetryPolicy(p), secOptions)
}

func (p *RetryPolicy) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return json.UnmarshalDecode(dec, (*plainRetryPolicy)(p), secOptions)
}

// A struct that others encode without options can choose the format of its time.Duration fields itself.
func ExampleWith_structWithoutOptions() {
	type Request struct {
		Name  string      `json:"name"`
		Retry RetryPolicy `json:"retry"`
	}

	req := Request{Name: "sync", Retry: RetryPolicy{MaxAttempts: 3, InitialBackoff: 250 * time.Millisecond}}

	b, err := json.Marshal(req)
	fmt.Println(string(b), err)

	// Output:
	// {"name":"sync","retry":{"max_attempts":3,"initial_backoff":0.25}} <nil>
}
