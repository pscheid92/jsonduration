package jsonduration_test

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"time"

	"github.com/pscheid92/jsonduration"
)

// daysOptions holds what the wrapper types of the package do not offer: a decode option.
var daysOptions = jsonduration.With(jsonduration.ISO8601, jsonduration.AcceptDays())

// DaysDuration is a wrapper type of your own. It encodes itself, as [jsonduration.ISO8601Duration] does, but with
// options of your choice.
type DaysDuration struct{ time.Duration }

func (d DaysDuration) MarshalJSONTo(enc *jsontext.Encoder) error {
	return json.MarshalEncode(enc, d.Duration, daysOptions)
}

func (d *DaysDuration) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return json.UnmarshalDecode(dec, &d.Duration, daysOptions)
}

// The wrapper types of the package are strict and take no options. If a field needs [jsonduration.AcceptDays] or
// [jsonduration.AlsoAccept], and you cannot pass options to the call, a wrapper type of your own takes a few lines.
func ExampleAcceptDays_ownWrapper() {
	type Job struct {
		Timeout DaysDuration `json:"timeout"`
	}

	var job Job
	if err := json.Unmarshal([]byte(`{"timeout":"P1DT12H"}`), &job); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(job.Timeout)

	b, err := json.Marshal(job)
	fmt.Println(string(b), err)

	// Output:
	// 36h0m0s
	// {"timeout":"PT36H"} <nil>
}
