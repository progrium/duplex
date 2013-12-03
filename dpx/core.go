package dpx

import (
	"bufio"
	"errors"
	"log"
	"net"
	"sync"
	"time"

	"github.com/hishboy/gocommons/lang"
	"github.com/ugorji/go/codec"
)

// This is the core API that would eventually be ported to C
// using libtask. As such, it's written somewhat more idiomatic
// to C than Go (though really a bastardization of both)

var mh codec.MsgpackHandle

var RetryWaitSec = 1

// Here is a shitty queue with blocking capabilities.
// Replace me
type Queue struct {
	Q  *lang.Queue
	ch chan struct{}
}

func NewQueue() *Queue {
	q := &Queue{Q: lang.NewQueue()}
	q.ch = make(chan struct{}, 1024)
	return q
}

func (q *Queue) Enqueue(item interface{}) {
	q.Q.Push(item)
	q.ch <- struct{}{}
}

func (q *Queue) Dequeue() interface{} {
	<-q.ch
	return q.Q.Poll()
}

type duplexconn struct {
	sync.Mutex
	conn     net.Conn
	writer   chan duplexframe
	channels map[int]duplexchannel
}

type duplexsocket struct {
	sync.Mutex
	listeners []net.Listener
	conns     []duplexconn
	outgoing  *Queue
	closed    bool
	rrIndex   int
	firstConn chan struct{}
}

type duplexchannel struct {
	incoming *Queue
}

const (
	OpenFrame   = 0
	InputFrame  = 1
	OutputFrame = 2
	ErrorFrame  = 3
)

var masterIncoming = NewQueue()

type duplexframe struct {
	_struct bool `codec:",toarray"`
	Type    int
	Channel int
	Headers map[string]string
	Payload []byte
}

func receiveGreeting(conn net.Conn) bool {
	// TODO
	return true
}

func sendGreeting(conn net.Conn) error {
	// TODO
	return nil
}

func addConnectionToSocket(socket *duplexsocket, conn net.Conn) {
	dconn := duplexconn{
		conn:     conn,
		writer:   make(chan duplexframe),
		channels: make(map[int]duplexchannel),
	}
	socket.Lock()
	socket.conns = append(socket.conns, dconn)
	if len(socket.conns) == 1 {
		socket.firstConn <- struct{}{}
	}
	socket.Unlock()
	go readFrames(dconn)
	go writeFrames(dconn)
}

func readFrames(conn duplexconn) {
	reader := bufio.NewReader(conn.conn)
	decoder := codec.NewDecoder(reader, &mh)
	for {
		var frame duplexframe
		err := decoder.Decode(&frame)
		if err != nil {
			log.Println(err)
		}
		if frame.Type == OpenFrame {
			if _, exists := conn.channels[frame.Channel]; !exists {
				conn.Lock()
				conn.channels[frame.Channel] = duplexchannel{incoming: masterIncoming}
				conn.Unlock()
			} else {
				log.Println("received open frame for channel that already exists")
			}
		} else {
			if _, exists := conn.channels[frame.Channel]; exists {
				conn.Lock()
				conn.channels[frame.Channel].incoming.Enqueue(frame)
				conn.Unlock()
			} else {
				log.Println("received frame for channel that doesn't exist")
			}
		}
	}
}

func writeFrames(conn duplexconn) {
	encoder := codec.NewEncoder(conn.conn, &mh)
	for {
		err := encoder.Encode(<-conn.writer)
		if err != nil {
			log.Println(err)
		}
	}
}

func routeFrames(socket *duplexsocket) {
	for {
		<-socket.firstConn
		for len(socket.conns) > 0 {
			frame := socket.outgoing.Dequeue().(duplexframe)
			socket.Lock()
			socket.conns[socket.rrIndex%len(socket.conns)].writer <- frame
			socket.rrIndex += 1
			socket.Unlock()
		}
		//time.Sleep(1 * time.Second)
	}
}

// Socket operations

func Socket() *duplexsocket {
	s := &duplexsocket{
		conns:     make([]duplexconn, 0),
		outgoing:  NewQueue(),
		firstConn: make(chan struct{}),
	}
	go routeFrames(s)
	return s
}

func Connect(socket *duplexsocket, addr string) error {
	if socket.closed {
		return errors.New("socket is closed")
	}
	go func() {
		for {
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				time.Sleep(time.Duration(RetryWaitSec) * time.Second)
				continue
			}
			if err := sendGreeting(conn); err != nil {
				log.Println("failed to make greeting")
				return
			}
			addConnectionToSocket(socket, conn)
			return
		}
	}()
	return nil
}

func Bind(socket *duplexsocket, addr string) error {
	if socket.closed {
		return errors.New("socket is closed")
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	socket.Lock()
	socket.listeners = append(socket.listeners, listener)
	socket.Unlock()
	go func() {
		for {
			conn, _ := listener.Accept()
			go func() {
				if receiveGreeting(conn) {
					addConnectionToSocket(socket, conn)
				}
			}()
		}
	}()
	return nil
}

func Close(socket *duplexsocket) error {
	socket.closed = true
	socket.Lock()
	defer socket.Unlock()
	for _, listener := range socket.listeners {
		if err := listener.Close(); err != nil {
			return err
		}
	}
	for _, conn := range socket.conns {
		if err := conn.conn.Close(); err != nil {
			return err
		}
	}
	return nil
}

func Auto(socket *duplexsocket, fn func() []string) error {
	return nil
}

func Codec(socket *duplexsocket, name string, codec interface{}) error {
	return nil
}

// Testing ...

func debugSend(socket *duplexsocket, frame duplexframe) {
	socket.outgoing.Enqueue(frame)
}

func debugReceive(socket *duplexsocket) duplexframe {
	return masterIncoming.Dequeue().(duplexframe)
}

// Channel operations

func Channel(socket *duplexsocket) *duplexchannel {
	return &duplexchannel{}
}

func Send(channel *duplexchannel, payload interface{}) error {
	return nil
}

func Receive(channel *duplexchannel, payload interface{}) error {
	return nil
}

func Senderr(channel *duplexchannel, payload interface{}) error {
	return nil
}

func Error(channel *duplexchannel, payload interface{}) error {
	return nil
}

// Client operations

func Open(socket *duplexsocket, resource string, channel *duplexchannel) error {
	return nil
}

func Call(socket *duplexsocket, resource string, arg interface{}, ret interface{}) error {
	return nil
}

// Server operations

func Register(socket *duplexsocket, resource string, receiver interface{}) error {
	return nil
}
