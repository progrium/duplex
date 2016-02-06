package dpx

import (
	"bufio"
	"errors"
	"net"
	"sync"
	"github.com/pborman/uuid"

	"github.com/ugorji/go/codec"
)

var RetryWaitSec = 1
var RetryAttempts = 20

func receiveGreeting(conn net.Conn) string {
	peerUUID := make([]byte, 36)
	read, err := conn.Read(peerUUID)

	if read != 36 {
		// FIXME does this mean the buffer hasn't filled or what?
		// treat this as invalid right now [and investigate]
		return ""
	}

	if err != nil {
		return ""
	}

	parsed := uuid.Parse(string(peerUUID))
	if parsed == nil {
		// invalid uuid; die
		return ""
	}

	debug("received greeting from ", conn.RemoteAddr(), ": ", parsed.String())

	// this is valid, return it
	return parsed.String()
}

func sendGreeting(conn net.Conn, uuid string) error {
	debug("sending greeting to ", conn.RemoteAddr(), ": ", uuid)
	bytes, err := conn.Write([]byte(uuid))
	if err != nil {
		return err
	}

	if bytes != 36 {
		// FIXME what
		return errors.New("dpx: failed to send greeting")
	}
	return nil
}

type duplexconn struct {
	sync.Mutex
	peer     *Peer
	conn     net.Conn
	writeCh  chan *Frame
	channels map[int]*Channel
	uuid     string
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
			c.peer.Drop(c.uuid)
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
