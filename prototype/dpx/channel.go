package dpx

import (
	"errors"
	"sync"
)

var ChannelQueueHWM = 1024

type Channel struct {
	sync.Mutex
	id       int
	peer     *Peer
	connCh   chan *duplexconn
	conn     *duplexconn
	server   bool
	closed   bool
	last     bool
	incoming chan *Frame
	outgoing chan *Frame
	err      error
	Method   string
}

func newClientChannel(p *Peer, method string) *Channel {
	channel := &Channel{
		peer:     p,
		connCh:   make(chan *duplexconn),
		incoming: make(chan *Frame, ChannelQueueHWM),
		outgoing: make(chan *Frame, ChannelQueueHWM),
		id:       p.chanIndex,
		Method:   method,
	}
	p.chanIndex += 1
	go channel.pumpOutgoing()
	return channel
}

func newServerChannel(conn *duplexconn, frame *Frame) *Channel {
	channel := &Channel{
		server:   true,
		connCh:   make(chan *duplexconn),
		peer:     conn.peer,
		incoming: make(chan *Frame, ChannelQueueHWM),
		outgoing: make(chan *Frame, ChannelQueueHWM),
		id:       frame.Channel,
		Method:   frame.Method,
	}
	go channel.pumpOutgoing()
	conn.LinkChannel(channel)
	return channel
}

func (c *Channel) close(err error) {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	if err != nil {
		c.err = err
	}
	c.conn.UnlinkChannel(c)
	close(c.connCh)
	close(c.incoming)
	close(c.outgoing)
}

func (c *Channel) Error() error {
	c.Lock()
	defer c.Unlock()
	return c.err
}

// not thread safe because of c.last?
func (c *Channel) ReceiveFrame() *Frame {
	if c.server && c.last {
		return nil
	}
	frame, ok := <-c.incoming
	if !ok {
		return nil
	}
	if frame.Last {
		if c.server {
			c.last = true
		} else {
			c.close(nil)
		}
	}
	return frame
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

func (c *Channel) Target() string {
	if c.conn == nil {
		return ""
	}

	return c.conn.uuid
}
