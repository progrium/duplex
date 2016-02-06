package dpx

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/pborman/uuid"
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
	uuid             string
}

func newPeer() *Peer {
	s := &Peer{
		index:            peerIndex,
		conns:            make([]*duplexconn, 0),
		openFrames:       make(chan *Frame, ChannelQueueHWM),
		incomingChannels: make(chan *Channel, 1024),
		firstConn:        make(chan struct{}),
		uuid:             uuid.New(),
	}
	peerIndex += 1
	go s.routeOpenFrames()
	return s
}

func (p *Peer) acceptConnection(conn net.Conn, uuid string) {
	dc := &duplexconn{
		peer:     p,
		conn:     conn,
		writeCh:  make(chan *Frame),
		channels: make(map[int]*Channel),
		uuid:     uuid,
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

			var conn *duplexconn
			var index int

			if frame.target != "" {
				for i, v := range s.conns {
					if v.uuid == frame.target {
						conn = v
						index = i
						break
					}
				}
			} else {
				conn, index = s.nextConn()
			}

			if conn == nil {
				panic("conn should not be nil")
			}

			debug(s.index, "Sending frame [", index, "]:", frame)
			err = conn.writeFrame(frame)
			if err == nil {
				conn.LinkChannel(frame.chanRef)
			}
		}
	}
}

func (p *Peer) Open(method string) *Channel {
	ret, err := p.OpenWith("", method)
	if err != nil {
		// which should not happen
		panic(err)
	}

	return ret
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
			if err := sendGreeting(conn, p.uuid); err != nil {
				debug("failed to make greeting")
				return
			}
			id := receiveGreeting(conn)
			if id == "" {
				debug("failed to receive remote greeting")
				return
			}
			debug(p.index, "Connected")
			p.acceptConnection(conn, id)
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
				if id := receiveGreeting(conn); id != "" {
					if err := sendGreeting(conn, p.uuid); err != nil {
						debug("failed to send greeting to incoming conn")
						return
					}
					p.acceptConnection(conn, id)
				}
			}()
		}
	}()
	return nil
}

func (p *Peer) Name() string {
	return p.uuid
}

func (p *Peer) Remote() []string {
	ret := make([]string, 0)
	for _, v := range p.conns {
		ret = append(ret, v.uuid)
	}

	return ret
}

func (p *Peer) OpenWith(uuid string, method string) (*Channel, error) {
	p.Lock()
	defer p.Unlock()
	if p.closed {
		return nil, errors.New("dpx: peer is already closed")
	}

	if uuid != "" {
		// verify that the uuid is valid
		found := false
		for _, v := range p.conns {
			if v.uuid == uuid {
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("dpx: no such peer with uuid")
		}
	}

	channel := newClientChannel(p, method)
	frame := newFrame(channel)
	frame.target = uuid
	frame.Type = OpenFrame
	frame.Method = method
	p.openFrames <- frame
	return channel, nil
}

func (p *Peer) Drop(uuid string) error {
	p.Lock()
	defer p.Unlock()
	if p.closed {
		return errors.New("dpx: peer is already closed")
	}

	// verify that the uuid is valid
	var conn *duplexconn
	var index int

	for i, v := range p.conns {
		if v.uuid == uuid {
			conn = v
			index = i
			break
		}
	}

	if conn == nil {
		return errors.New("dpx: no such peer with uuid")
	}

	for _, v := range conn.channels {
		v.close(errors.New("dpx: connection dropped"))
	}

	conn.conn.Close()
	// I don't check for io.EOF because it could be something else, too...

	p.conns = append(p.conns[:index], p.conns[index+1:]...)
	return nil
}
