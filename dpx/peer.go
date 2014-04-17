package dpx

import (
	"log"
	"net"
	"sync"
)

var peerIndex = 0

type Peer struct {
	sync.Mutex
	listeners      []net.Listener
	conns          []*duplexconn
	routableFrames *queue
	channelQueue   *queue
	closed         bool
	rrIndex        int
	chanIndex      int
	index          int
	firstConn      chan struct{}
}

func newPeer() *Peer {
	s := &Peer{
		index:          peerIndex,
		conns:          make([]*duplexconn, 0),
		routableFrames: newQueue(),
		channelQueue:   newQueue(),
		firstConn:      make(chan struct{}),
	}
	peerIndex += 1
	go s.routeFrames()
	return s
}

func (s *Peer) addConnection(conn net.Conn) {
	dconn := &duplexconn{
		peer:     s,
		conn:     conn,
		writeCh:  make(chan *Frame),
		channels: make(map[int]*Channel),
	}
	s.Lock()
	s.conns = append(s.conns, dconn)
	if len(s.conns) == 1 {
		s.firstConn <- struct{}{}
	}
	s.Unlock()
	go dconn.readFrames()
	go dconn.writeFrames()
}

func (s *Peer) nextConn() (*duplexconn, int) {
	index := s.rrIndex % len(s.conns)
	s.Lock()
	conn := s.conns[index]
	s.rrIndex += 1
	s.Unlock()
	return conn, index
}

func (s *Peer) routeFrames() {
	var err error
	var frame *Frame
	for {
		<-s.firstConn
		log.Println(s.index, "First connection, routing...")
		for len(s.conns) > 0 {
			if err == nil {
				frame = s.routableFrames.Dequeue().(*Frame)
			}
			conn, index := s.nextConn()
			log.Println(s.index, "Sending frame [", index, "]:", frame)
			err = conn.writeFrame(frame)
			if err == nil {
				conn.addChannel(frame.chanRef)
			}
		}
	}
}

func (s *Peer) newChannel() *Channel {
	channel := &Channel{
		peer:     s,
		incoming: newQueue(),
		outgoing: newQueue(),
		id:       s.chanIndex,
	}
	s.chanIndex += 1
	return channel
}

func (s *Peer) accept() *Channel {
	return s.channelQueue.Dequeue().(*Channel)
}
