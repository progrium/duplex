package dpx

import (
	"bufio"
	"errors"
	"log"
	"net"
	"sync"
	"time"

	"github.com/ugorji/go/codec"
)

var RetryWaitSec = 1

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
			log.Println(err)
			return
		}
		log.Println("Reading frame:", frame)
		c.Lock()
		ch, exists := c.channels[frame.Channel]
		c.Unlock()
		if frame.Type == OpenFrame {
			if !exists {
				channel := c.peer.newChannel()
				channel.server = true
				channel.id = frame.Channel
				channel.method = frame.Method
				c.addChannel(channel)
				c.peer.channelQueue.Enqueue(channel)
			} else {
				log.Println("received open frame for channel that already exists")
			}
		} else {
			if exists {
				if frame.Error != "" {
					log.Println("received error: ", frame.Error)
					ch.err = errors.New(frame.Error)
					ch.outgoing.Close()
					ch.incoming.Close()
					c.removeChannel(ch)
					continue
				}
				if !ch.incoming.Draining() {
					ch.incoming.Enqueue(&frame)
					if frame.Last {
						ch.incoming.EnqueueLast(nil)
					}
				} else {
					log.Println("received frame for channel that is already finished")
				}
			} else {
				log.Println("received frame for channel that doesn't exist")
			}
		}
	}
}

func (c *duplexconn) writeFrames() {
	encoder := codec.NewEncoder(c.conn, &mh)
	for {
		frame := <-c.writeCh
		log.Println("Writing frame:", frame)
		err := encoder.Encode(frame)
		if err != nil {
			log.Println(err)
		}
		frame.errCh <- err
		log.Println("Done writing")
	}
}

func (c *duplexconn) writeFrame(frame *Frame) error {
	c.writeCh <- frame
	return <-frame.errCh
}

func (c *duplexconn) addChannel(ch *Channel) {
	c.Lock()
	defer c.Unlock()
	c.channels[ch.id] = ch
	ch.conn = c
	go ch.pumpFrames()
}

func (c *duplexconn) removeChannel(ch *Channel) {
	c.Lock()
	defer c.Unlock()
	delete(c.channels, ch.id)
}

func (s *Peer) connect(addr string) error {
	if s.closed {
		return errors.New("peer is closed")
	}
	go func() {
		for {
			log.Println(s.index, "Connecting", addr)
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				log.Println(s.index, "Unable to connect, retrying soon...")
				time.Sleep(time.Duration(RetryWaitSec) * time.Second)
				continue
			}
			if err := sendGreeting(conn); err != nil {
				log.Println("failed to make greeting")
				return
			}
			log.Println(s.index, "Connected")
			s.addConnection(conn)
			return
		}
	}()
	return nil
}

func (s *Peer) bind(addr string) error {
	if s.closed {
		return errors.New("peer is closed")
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.Lock()
	s.listeners = append(s.listeners, listener)
	s.Unlock()
	log.Println(s.index, "Now listening", addr)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Println(err.Error())
				return
			}
			go func() {
				if receiveGreeting(conn) {
					s.addConnection(conn)
				}
			}()
		}
	}()
	return nil
}

func (s *Peer) close() error {
	s.closed = true
	s.Lock()
	defer s.Unlock()
	for _, listener := range s.listeners {
		if err := listener.Close(); err != nil {
			return err
		}
	}
	for _, conn := range s.conns {
		if err := conn.conn.Close(); err != nil {
			return err
		}
	}
	return nil
}
