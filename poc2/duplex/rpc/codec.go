package rpc

import (
	"bytes"
	"io"

	"github.com/ugorji/go/codec"
)

var mh codec.MsgpackHandle

func codecDecode(frame []byte, obj interface{}) error {
	if frame == nil {
		return io.EOF
	}
	buffer := bytes.NewBuffer(frame)
	decoder := codec.NewDecoder(buffer, &mh)
	return decoder.Decode(obj)
}

func codecEncode(obj interface{}) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := codec.NewEncoder(&buffer, &mh)
	err := encoder.Encode(obj)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
