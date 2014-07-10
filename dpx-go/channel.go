package dpx

// #cgo LDFLAGS: -ldpx
// #include <dpx.h>
import "C"

import (
	"errors"
	"runtime"
)

var ChannelQueueHWM = 1024

type Channel struct {
	ch *C.dpx_channel
}

func fromCChannel(ch *C.dpx_channel) *Channel {
	channel := &Channel{ch: ch}
	runtime.SetFinalizer(channel, func(x *Channel) {
		C.dpx_channel_free(x.ch)
	})
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
	c.Lock()
	defer c.Unlock()
	if c.err != nil {
		return c.err
	}
	if c.closed {
		return errors.New("duplex: channel is closed")
	}
	frame.chanRef = c
	frame.Channel = c.id
	frame.Type = DataFrame
	c.outgoing <- frame
	return nil
}

func (c *Channel) HandleIncoming(frame *Frame) bool {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return false
	}
	if !frame.Last && frame.Error != "" {
		go c.close(errors.New(frame.Error))
		return true
	}
	c.incoming <- frame
	return true
}

func (c *Channel) pumpOutgoing() {
	var stillopen bool
	c.conn = <-c.connCh
	debug(c.peer.index, "Pumping started for channel", c.id)
	defer debug(c.peer.index, "Pumping finished for channel", c.id)
	for {
		select {
		case c.conn, stillopen = <-c.connCh:
			if !stillopen {
				c.conn = nil
				return
			}
		case frame, ok := <-c.outgoing:
			if !ok {
				return
			}
			for {
				debug(c.peer.index, "Sending frame:", frame)
				err := c.conn.writeFrame(frame)
				if err != nil {
					debug(c.peer.index, "Error sending frame:", err)
					c.conn, stillopen = <-c.connCh
					debug(c.peer.index, "New connection for channel", c.id)
					if !stillopen {
						c.conn = nil
						return
					}
					continue
				}
				if frame.Error != "" {
					c.close(errors.New(frame.Error))
				} else if frame.Last && c.server {
					c.close(nil)
				}
				break
			}
		}
	}
}
