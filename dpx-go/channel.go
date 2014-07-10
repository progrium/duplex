package dpx

// #cgo LDFLAGS: -ldpx
// #include <dpx.h>
import "C"

import "runtime"

var ChannelQueueHWM = 1024

type Channel struct {
	ch *C.dpx_channel
}

func fromCChannel(ch *C.dpx_channel) *Channel {
	channel := &Channel{ch: ch}
	runtime.SetFinalizer(channel, func(x *Channel) {
		C.dpx_channel_free(x.ch)
	})
	return channel
}

func (c *Channel) Method() string {
	return C.GoString(C.dpx_channel_method_get(c.ch))
}

func (c *Channel) Error() error {
	return ParseError(uint64(C.dpx_channel_error(c.ch)))
}

func (c *Channel) ReceiveFrame() *Frame {
	frame := C.dpx_channel_receive_frame(c.ch)

	ourframe := fromCFrame(frame)
	C.dpx_frame_free(frame)
	return ourframe
}

func (c *Channel) SendFrame(frame *Frame) error {
	return ParseError(uint64(C.dpx_channel_send_frame(c.ch, toCFrame(frame))))
}
