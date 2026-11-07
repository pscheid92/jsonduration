package jsonduration

import (
	"encoding/json/jsontext"
	"time"
)

// UnitsDuration holds a [time.Duration] and encodes it in format [Units], such as "1h30m0.5s", without options.
type UnitsDuration struct{ time.Duration }

// MarshalJSONTo implements [encoding/json/v2.MarshalerTo].
func (d UnitsDuration) MarshalJSONTo(enc *jsontext.Encoder) error {
	return encode(enc, codecUnits, d.Duration)
}

// UnmarshalJSONFrom implements [encoding/json/v2.UnmarshalerFrom].
func (d *UnitsDuration) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return decoderUnits.decode(dec, &d.Duration)
}

// SecDuration holds a [time.Duration] and encodes it in format [Sec], such as 5400.5, without options.
type SecDuration struct{ time.Duration }

// MarshalJSONTo implements [encoding/json/v2.MarshalerTo].
func (d SecDuration) MarshalJSONTo(enc *jsontext.Encoder) error {
	return encode(enc, codecSec, d.Duration)
}

// UnmarshalJSONFrom implements [encoding/json/v2.UnmarshalerFrom].
func (d *SecDuration) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return decoderSec.decode(dec, &d.Duration)
}

// MilliDuration holds a [time.Duration] and encodes it in format [Milli], such as 5400500, without options.
type MilliDuration struct{ time.Duration }

// MarshalJSONTo implements [encoding/json/v2.MarshalerTo].
func (d MilliDuration) MarshalJSONTo(enc *jsontext.Encoder) error {
	return encode(enc, codecMilli, d.Duration)
}

// UnmarshalJSONFrom implements [encoding/json/v2.UnmarshalerFrom].
func (d *MilliDuration) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return decoderMilli.decode(dec, &d.Duration)
}

// MicroDuration holds a [time.Duration] and encodes it in format [Micro], such as 5400500000, without options.
type MicroDuration struct{ time.Duration }

// MarshalJSONTo implements [encoding/json/v2.MarshalerTo].
func (d MicroDuration) MarshalJSONTo(enc *jsontext.Encoder) error {
	return encode(enc, codecMicro, d.Duration)
}

// UnmarshalJSONFrom implements [encoding/json/v2.UnmarshalerFrom].
func (d *MicroDuration) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return decoderMicro.decode(dec, &d.Duration)
}

// NanoDuration holds a [time.Duration] and encodes it in format [Nano], such as 5400500000000, without options.
type NanoDuration struct{ time.Duration }

// MarshalJSONTo implements [encoding/json/v2.MarshalerTo].
func (d NanoDuration) MarshalJSONTo(enc *jsontext.Encoder) error {
	return encode(enc, codecNano, d.Duration)
}

// UnmarshalJSONFrom implements [encoding/json/v2.UnmarshalerFrom].
func (d *NanoDuration) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return decoderNano.decode(dec, &d.Duration)
}

// ISO8601Duration holds a [time.Duration] and encodes it in format [ISO8601], such as "PT1H30M0.5S", without options.
type ISO8601Duration struct{ time.Duration }

// MarshalJSONTo implements [encoding/json/v2.MarshalerTo].
func (d ISO8601Duration) MarshalJSONTo(enc *jsontext.Encoder) error {
	return encode(enc, codecISO8601, d.Duration)
}

// UnmarshalJSONFrom implements [encoding/json/v2.UnmarshalerFrom].
func (d *ISO8601Duration) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return decoderISO8601.decode(dec, &d.Duration)
}

// The decoders of the wrapper types, one per format.
var (
	decoderUnits   = newDecoder(Units, nil)
	decoderSec     = newDecoder(Sec, nil)
	decoderMilli   = newDecoder(Milli, nil)
	decoderMicro   = newDecoder(Micro, nil)
	decoderNano    = newDecoder(Nano, nil)
	decoderISO8601 = newDecoder(ISO8601, nil)
)
