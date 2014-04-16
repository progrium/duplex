package dpx

import (
	"bufio"
	"bytes"
	"errors"
	"log"
	"net"
	"sync"
	"time"

	"github.com/hishboy/gocommons/lang"
	"github.com/ugorji/go/codec"
)

// This is the core API that would eventually be ported to C
// using libtask.

// The public API methods at the bottom would be exposed in
// a C library as dpx_send, dpx_connect, etc

var mh codec.MsgpackHandle

var RetryWaitSec = 1

var peerIndex = 0

// Here is a shitty queue with blocking capabilities.
// Replace me
type queue struct {
	Q        *lang.Queue
	ch       chan struct{}
	draining bool
	finished bool
}

func newQueue() *queue {
	q := &queue{Q: lang.NewQueue()}
	q.ch = make(chan struct{}, 1024)
	return q
}

func (q *queue) Enqueue(item interface{}) error {
	if q.draining {
		return errors.New("queue already received last")
	}
	q.Q.Push(item)
	q.ch <- struct{}{}
	return nil
}

func (q *queue) EnqueueLast(item interface{}) (err error) {
	err = q.Enqueue(item)
	q.draining = true
	return
}

func (q *queue) Dequeue() interface{} {
	<-q.ch
	return q.Q.Poll()
}

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
		if frame.Type == OpenFrame {
			if _, exists := c.channels[frame.Channel]; !exists {
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
			if ch, exists := c.channels[frame.Channel]; exists {
				c.Lock()
				if !ch.incoming.draining {
					ch.incoming.Enqueue(&frame)
					if frame.Last {
						ch.incoming.EnqueueLast(nil)
					}
				} else {
					log.Println("received frame for channel that is already finished")
				}
				c.Unlock()
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
	registered     map[string]*duplexmethod
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

type Channel struct {
	id       int
	peer     *Peer
	conn     *duplexconn
	server   bool
	incoming *queue
	outgoing *queue
	method   string
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

const (
	OpenFrame = 0
	DataFrame = 1
)

type Frame struct {
	_struct bool `codec:",toarray"`
	errCh   chan error
	chanRef *Channel

	Type    int
	Channel int

	Method  string
	Headers map[string]string

	Payload []byte
	Error   string
	Last    bool
}

type duplexmethod struct {
	name    string
	handler *interface{}
	doc     string
}

// PUBLIC C-like API

// Peer operations

func NewPeer() *Peer {
	return newPeer()
}

func Connect(peer *Peer, addr string) error {
	return peer.connect(addr)
}

func Bind(peer *Peer, addr string) error {
	return peer.bind(addr)
}

func Close(peer *Peer) error {
	return peer.close()
}

func Auto(peer *Peer, fn func() []string) error {
	return nil // TODO
}

func Codec(peer *Peer, name string, codec interface{}) error {
	return nil // TODO
}

// Channel operations

func NewFrame(channel *Channel) *Frame {
	return channel.newFrame()
}

func SendFrame(channel *Channel, frame *Frame) error {
	frame.chanRef = channel
	frame.Channel = channel.id
	frame.Type = DataFrame
	if frame.Last {
		return channel.outgoing.EnqueueLast(frame)
	} else {
		return channel.outgoing.Enqueue(frame)
	}
}

// blocks
func ReceiveFrame(channel *Channel) *Frame {
	if channel.incoming.finished {
		return nil
	}
	frame := channel.incoming.Dequeue().(*Frame)
	if channel.incoming.draining && frame == nil {
		channel.incoming.finished = true
		// ok to remove since only affects incoming frames
		channel.conn.removeChannel(channel)
	}
	return frame
}

func Send(channel *Channel, data interface{}) error {
	req := NewFrame(channel)
	Encode(channel, req, data)
	return SendFrame(channel, req)
}

func SendLast(channel *Channel, data interface{}) error {
	req := NewFrame(channel)
	if data != nil {
		Encode(channel, req, data)
	}
	req.Last = true
	return SendFrame(channel, req)
}

func SendErr(channel *Channel, err string) error {
	req := NewFrame(channel)
	req.Error = err
	req.Last = true
	return SendFrame(channel, req)
}

func Receive(channel *Channel, obj interface{}) error {
	return Decode(channel, ReceiveFrame(channel), obj)
}

func Decode(ch *Channel, frame *Frame, obj interface{}) error {
	if frame.Error != "" {
		return errors.New("remote: " + frame.Error)
	}
	buffer := bytes.NewBuffer(frame.Payload)
	decoder := codec.NewDecoder(buffer, &mh)
	return decoder.Decode(obj)
}

func Encode(ch *Channel, frame *Frame, obj interface{}) error {
	buffer := new(bytes.Buffer)
	encoder := codec.NewEncoder(buffer, &mh)
	err := encoder.Encode(obj)
	if err != nil {
		return err
	}
	frame.Payload = buffer.Bytes()
	return nil
}

// Client operations

func Open(peer *Peer, method string) *Channel {
	channel := peer.newChannel()
	channel.method = method
	frame := channel.newFrame()
	frame.Type = OpenFrame
	frame.Method = method
	peer.routableFrames.Enqueue(frame)
	return channel
}

// blocks
func Call(peer *Peer, method string, arg interface{}) interface{} {
	ch := Open(peer, method)
	req := NewFrame(ch)
	req.Payload = arg.([]byte)
	req.Last = true
	SendFrame(ch, req)
	resp := ReceiveFrame(ch)
	return resp.Payload
}

// Server operations

func Register(peer *Peer, method string, doc string, handler *interface{}) {
	peer.Lock()
	defer peer.Unlock()
	peer.registered[method] = &duplexmethod{
		name:    method,
		doc:     doc,
		handler: handler,
	}
}

// blocks
func Accept(peer *Peer) (string, *Channel) {
	ch := peer.accept()
	return ch.method, ch
}
