package dpx

import (
	"encoding/json"
	"io"
)

type Codec interface {
	Decoder(r io.Reader) Decoder
	Encoder(w io.Writer) Encoder
}

type Decoder interface {
	Decode(v interface{}) error
}

type Encoder interface {
	Encode(v interface{}) error
}

type JSONCodec struct{}

func (j *JSONCodec) Decoder(r io.Reader) Decoder {
	return json.NewDecoder(r)
}

func (j *JSONCodec) Encoder(w io.Writer) Encoder {
	return json.NewEncoder(w)
}
