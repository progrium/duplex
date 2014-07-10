package dpx

import (
	"bufio"
	"net"
	"sync"

	"github.com/ugorji/go/codec"
)

var RetryWaitSec = 1
var RetryAttempts = 20

func receiveGreeting(conn net.Conn) bool {
	// TODO
	return true
}

func sendGreeting(conn net.Conn) error {
	// TODO
	return nil
}

type duplexconn struct {
	sync.Mutex
	peer     *Peer
	conn     net.Conn
	writeCh  chan *Frame
	channels map[int]*Channel
}

func (c *duplexconn) readFrames() {
	reader := bufio.NewReader(c.conn)
	decoder := codec.NewDecoder(reader, &mh)
	for {
		var frame Frame
		err := decoder.Decode(&frame)
		if err != nil {
			debug(err)
			close(c.writeCh)
			return
		}
		debug("Reading frame:", frame)
		c.Lock()
		ch, exists := c.channels[frame.Channel]
		c.Unlock()
		if exists && frame.Type == DataFrame {
			if ch.HandleIncoming(&frame) {
				continue
			}
		}
		if !exists && frame.Type == OpenFrame {
			if c.peer.HandleOpen(c, &frame) {
				continue
			}
		}
		debug("Dropped frame:", frame)
	}
}

func (c *duplexconn) writeFrames() {
	encoder := codec.NewEncoder(c.conn, &mh)
	for {
		frame, ok := <-c.writeCh
		if !ok {
			return
		}
		err := encoder.Encode(frame)
		frame.errCh <- err
		if err != nil {
			debug(err)
			return
		}
	}
}

func (c *duplexconn) writeFrame(frame *Frame) error {
	c.writeCh <- frame
	return <-frame.errCh
}

func (c *duplexconn) LinkChannel(ch *Channel) {
	c.Lock()
	defer c.Unlock()
	c.channels[ch.id] = ch
	ch.connCh <- c
}

func (c *duplexconn) UnlinkChannel(ch *Channel) {
	c.Lock()
	defer c.Unlock()
	delete(c.channels, ch.id)
}
