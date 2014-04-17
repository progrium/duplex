package dpx

import (
	"log"
)

type Channel struct {
	id       int
	peer     *Peer
	conn     *duplexconn
	server   bool
	incoming *queue
	outgoing *queue
	method   string
	err      error
}

func (c *Channel) newFrame() *Frame {
	return &Frame{
		chanRef: c,
		Channel: c.id,
		Headers: map[string]string{},
		errCh:   make(chan error),
	}
}

func (c *Channel) pumpFrames() {
	for {
		if c.conn == nil {
			log.Println(c.peer.index, "No connection, pumping stopped")
			return
		}
		frame := c.outgoing.Dequeue().(*Frame)
		log.Println(c.peer.index, "Sending frame:", frame)
		err := c.conn.writeFrame(frame)
		if err != nil {
			log.Println(c.peer.index, "Error sending frame:", err.Error())
			c.conn = nil
		}
	}
}
