package dpx

// #cgo LDFLAGS: -ldpx -lmsgpack
// #include <dpx.h>
import "C"

import (
	"time"

	"runtime"
)

var ChannelQueueHWM = 1024

type Channel struct {
	ch *C.dpx_channel
}

func fromCChannel(ch *C.dpx_channel) *Channel {
	if ch == nil {
		return nil
	}

	channel := &Channel{ch: ch}
	runtime.SetFinalizer(channel, func(x *Channel) {
		C.dpx_channel_close(x.ch, 1)
		go func() {
			time.Sleep(500 * time.Millisecond)
			C.dpx_channel_free(x.ch)
		}()
	})
	return channel
}

func (c *Channel) Method() string {
	return C.GoString(C.dpx_channel_method_get(c.ch))
}

func (c *Channel) Error() error {
	return ParseError(int64(C.dpx_channel_error(c.ch)))
}

func (c *Channel) ReceiveFrame() *Frame {
	frame := C.dpx_channel_receive_frame(c.ch)

	if frame == nil {
		return nil
	}

	ourframe := fromCFrame(frame)
	C.dpx_frame_free(frame)
	return ourframe
}

func (c *Channel) SendFrame(frame *Frame) error {
	cframe := toCFrame(frame)
	err := ParseError(int64(C.dpx_channel_send_frame(c.ch, cframe)))
	C.dpx_frame_free(cframe)
	return err
}
