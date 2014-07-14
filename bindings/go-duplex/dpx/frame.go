package dpx

// #cgo LDFLAGS: -ldpx
// #include "frame.h"
import "C"
import (
	"unsafe"
)

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
		Error:   C.GoString(frame.error),
		Last:    (frame.last != 0),
		Payload: C.GoBytes(unsafe.Pointer(frame.payload), frame.payloadSize),
	}

	// iterating through headers:
	C.dpx_frame_header_iter(frame, (*[0]byte)(C.header_helper), unsafe.Pointer(f))

	return f
}

//export helperAdd
func helperAdd(p *unsafe.Pointer, k *C.char, v *C.char) {
	frame := (*Frame)(unsafe.Pointer(p))
	frame.Headers[C.GoString(k)] = C.GoString(v)
}

func toCFrame(frame *Frame) *C.dpx_frame {
	cframe := C.dpx_frame_new(nil)
	cframe.method = C.CString(frame.Method)
	cframe.headers = nil
	cframe.error = C.CString(frame.Error)
	if frame.Last {
		cframe.last = C.int(1)
	} else {
		cframe.last = C.int(0)
	}

	for k, v := range frame.Headers {
		C.dpx_frame_header_add(cframe, C.CString(k), C.CString(v))
	}

	C.convert_payload(cframe, unsafe.Pointer(&frame.Payload[0]), C.int(len(frame.Payload)))

	return cframe
}
