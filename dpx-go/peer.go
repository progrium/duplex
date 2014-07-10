package dpx

import (
	"errors"
	"net"
	"sync"
	"time"
)

var peerIndex = 0

type Peer struct {
	sync.Mutex
	listeners        []net.Listener
	conns            []*duplexconn
	openFrames       chan *Frame
	incomingChannels chan *Channel
	closed           bool
	rrIndex          int
	chanIndex        int
	index            int
	firstConn        chan struct{}
}

func newPeer() *Peer {
	s := &Peer{
		index:            peerIndex,
		conns:            make([]*duplexconn, 0),
		openFrames:       make(chan *Frame, ChannelQueueHWM),
		incomingChannels: make(chan *Channel, 1024),
		firstConn:        make(chan struct{}),
	}
	peerIndex += 1
	go s.routeOpenFrames()
	return s
}

func (p *Peer) acceptConnection(conn net.Conn) {
	dc := &duplexconn{
		peer:     p,
		conn:     conn,
		writeCh:  make(chan *Frame),
		channels: make(map[int]*Channel),
	}
	p.Lock()
	p.conns = append(p.conns, dc)
	if len(p.conns) == 1 {
		p.firstConn <- struct{}{}
	}
	p.Unlock()
	go dc.readFrames()
	go dc.writeFrames()
}

func (s *Peer) nextConn() (*duplexconn, int) {
	index := s.rrIndex % len(s.conns)
	s.Lock()
	conn := s.conns[index]
	s.rrIndex += 1
	s.Unlock()
	return conn, index
}

func (s *Peer) routeOpenFrames() {
	var err error
	var frame *Frame
	var ok bool
	for {
		<-s.firstConn
		debug(s.index, "First connection, routing...")
		for len(s.conns) > 0 {
			if err == nil {
				frame, ok = <-s.openFrames
				if !ok {
					return
				}
			}
			conn, index := s.nextConn()
			debug(s.index, "Sending frame [", index, "]:", frame)
			err = conn.writeFrame(frame)
			if err == nil {
				conn.LinkChannel(frame.chanRef)
			}
		}
	}
}

func (p *Peer) Open(method string) *Channel {
	p.Lock()
	defer p.Unlock()
	if p.closed {
		return nil
	}
	channel := newClientChannel(p, method)
	frame := newFrame(channel)
	frame.Type = OpenFrame
	frame.Method = method
	p.openFrames <- frame
	return channel
}

func (p *Peer) HandleOpen(conn *duplexconn, frame *Frame) bool {
	p.Lock()
	defer p.Unlock()
	if p.closed {
		return false
	}
	p.incomingChannels <- newServerChannel(conn, frame)
	return true
}

func (p *Peer) Accept() *Channel {
	channel, ok := <-p.incomingChannels
	if !ok {
		return nil
	}
	return channel
}

func (p *Peer) Close() error {
	p.Lock()
	defer p.Unlock()
	if p.closed {
		return errors.New("duplex: peer already closed")
	}
	p.closed = true
	close(p.openFrames)
	close(p.incomingChannels)
	for _, listener := range p.listeners {
		if err := listener.Close(); err != nil {
			return err
		}
	}
	for _, conn := range p.conns {
		if err := conn.conn.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (p *Peer) Connect(addr string) error {
	p.Lock()
	defer p.Unlock()
	if p.closed {
		return errors.New("duplex: peer is closed")
	}
	go func() {
		for i := 0; i < RetryAttempts; i++ {
			debug(p.index, "Connecting", addr)
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				debug(p.index, "Unable to connect, retrying soon...")
				time.Sleep(time.Duration(RetryWaitSec) * time.Second)
				continue
			}
			if err := sendGreeting(conn); err != nil {
				debug("failed to make greeting")
				return
			}
			debug(p.index, "Connected")
			p.acceptConnection(conn)
			return
		}
	}()
	return nil
}

func (p *Peer) Bind(addr string) error {
	p.Lock()
	defer p.Unlock()
	if p.closed {
		return errors.New("duplex: peer is closed")
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	p.listeners = append(p.listeners, listener)
	debug(p.index, "Now listening", addr)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				debug(err)
				return
			}
			go func() {
				if receiveGreeting(conn) {
					p.acceptConnection(conn)
				}
			}()
		}
	}()
	return nil
}
