package dpx

// #cgo LDFLAGS: -ldpx
// #include <dpx.h>
// #include <stdlib.h>
// #include <string.h>
// #include "uthash.h"
//
// void
// add_header(dpx_frame *f, char* key, char* val)
// {
//		dpx_header_map *m = malloc(sizeof(dpx_header_map));
//		m->key = key;
//		m->value = value;
//		HASH_ADD_KEYPTR(hh, f->headers, m->key, strlen(m->key), m);
// }
//
// void
// convert_payload(dpx_frame *f, char* payload, int payload_size)
// {
//		f->payload = malloc(payload_size);
//		f->payloadSize = payload_size;
//		memmove(f->payload, payload, payload_size);
// }
import "C"

const (
	OpenFrame = 0
	DataFrame = 1
)

type Frame struct {
	Method  string
	Headers map[string]string
	Error   string
	Last    bool

	Payload []byte
}

func fromCFrame(frame *C.dpx_frame) *Frame {
	f := &Frame{
		Method:  C.GoString(frame.method),
		Headers: make(map[string]string),
		Error:   C.GoString(frame._error),
		Last:    (frame.last != 0),
		Payload: C.GoBytes(frame.payload, frame.payloadSize),
	}

	// iterating through headers:
	for s := frame.headers; s != nil; s = s.hh.next {
		f.Headers[C.GoString(s.key)] = C.GoString(s.value)
	}

	return f
}

func toCFrame(frame *Frame) *C.dpx_frame {
	cframe := C.dpx_frame_new(nil)
	cframe.method = C.CString(frame.Method)
	cframe.headers = nil
	cframe._error = C.CString(frame.Error)
	if frame.Last {
		cframe.last = C.int(1)
	} else {
		cframe.last = C.int(0)
	}

	for k, v := range frame.Headers {
		C.add_header(cframe, C.CString(k), C.CString(v))
	}

	C.convert_payload(cframe, &frame.Payload[0], len(frame.Payload))

	return cframe
}
