package simplex

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
)

/*
Notes

The io.ReadWriteCloser is assumed to be a *framed*
transport. So we expect each Read to be a full frame.
The buffer provided to Read needs to be big enough,
which means using a variable-sized Buffer.

We also assume the io.ReadWriteCloser to handle locking
on Writes.

Potential requirement on spec: msg id has to be 1 or more

TODO: replace panics

*/

var (
	Version         = "0.1.0"
	ProtocolName    = "SIMPLEX"
	ProtocolVersion = "1.0"
	TypeRequest     = "req"
	TypeReply       = "rep"
	HandshakeAccept = "+OK"
	BacklogSize     = 1024
	MaxFrameSize    = 1024 * 8 // 8kb
)

type Message struct {
	Type    string      `json:"type"`
	Method  string      `json:"method,omitempty"`
	Payload interface{} `json:"payload,omitempty"`
	Error   *Error      `json:"error,omitempty"`
	Id      int         `json:"id,omitempty"`
	More    bool        `json:"more,omitempty"`
	Ext     interface{} `json:"ext,omitempty"`
}

type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (err Error) Error() string {
	return err.Message
}

type Codec struct {
	Name   string
	Encode func(obj interface{}) ([]byte, error)
	Decode func(frame []byte, obj interface{}) error
}

func NewJSONCodec() *Codec {
	return &Codec{
		Name:   "json",
		Encode: json.Marshal,
		Decode: json.Unmarshal,
	}
}

type RPC struct {
	codec      *Codec
	registered map[string]func(*Channel) error
}

func NewRPC(codec *Codec) *RPC {
	return &RPC{
		codec:      codec,
		registered: make(map[string]func(*Channel) error),
	}
}

func (rpc *RPC) Register(name string, fn func(*Channel) error) {
	rpc.registered[name] = fn
}

func (rpc *RPC) Handshake(conn io.ReadWriteCloser) (*Peer, error) {
	peer := NewPeer(rpc, conn)
	handshake := []byte(fmt.Sprintf("%s/%s;%s",
		ProtocolName, ProtocolVersion, rpc.codec.Name))
	_, err := conn.Write(handshake)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 32)
	_, err = conn.Read(buf)
	if err != nil {
		return nil, err
	}
	if buf[0] != '+' {
		panic(string(buf))
	}
	go peer.route()
	return peer, nil
}

func (rpc *RPC) Accept(conn io.ReadWriteCloser) (*Peer, error) {
	peer := NewPeer(rpc, conn)
	buf := make([]byte, 32)
	_, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	// TODO: check handshake
	_, err = conn.Write([]byte(HandshakeAccept))
	if err != nil {
		return nil, err
	}
	go peer.route()
	return peer, nil
}

type Peer struct {
	counter int
	reqCh   map[int]*Channel
	repCh   map[int]*Channel
	rpc     *RPC
	conn    io.ReadWriteCloser
	closeCh chan bool
}

func NewPeer(rpc *RPC, conn io.ReadWriteCloser) *Peer {
	peer := &Peer{
		rpc:     rpc,
		conn:    conn,
		reqCh:   make(map[int]*Channel),
		repCh:   make(map[int]*Channel),
		closeCh: make(chan bool),
	}
	return peer
}

func (peer *Peer) CloseNotify() <-chan bool {
	return peer.closeCh
}

func (peer *Peer) route() {
	// assumes closing will cause something
	// here to error and break loop.
	// TODO: double check
	for {
		frame := make([]byte, MaxFrameSize)
		n, err := peer.conn.Read(frame)
		if err != nil {
			// TODO: what happens on read error
			break
		}
		var msg Message
		err = peer.rpc.codec.Decode(frame[:n], &msg)
		if err != nil {
			// TODO: what happens on decode error
			panic(err)
		}
		switch msg.Type {
		case TypeRequest:
			ch, exists := peer.reqCh[msg.Id]
			if exists {
				if !msg.More {
					delete(peer.reqCh, msg.Id)
				}
			} else {
				ch = NewChannel(peer, TypeReply, msg.Method)
				if msg.Id != 0 {
					ch.id = msg.Id
					if msg.More {
						peer.reqCh[ch.id] = ch
					}
				}
				fn, exists := peer.rpc.registered[msg.Method]
				if exists {
					go fn(ch)
				} else {
					// TODO: method missing
					panic(msg.Method)
				}
			}
			if msg.Ext != nil {
				ch.ext = msg.Ext
			}
			ch.inbox <- &msg
			if !msg.More {
				close(ch.inbox)
			}

		case TypeReply:
			if msg.Error != nil {
				// TODO: better handling of missing id
				ch := peer.repCh[msg.Id]
				ch.err = msg.Error
				ch.done <- ch
				close(ch.inbox)
				delete(peer.repCh, msg.Id)
			} else {
				// TODO: better handling of missing id
				ch := peer.repCh[msg.Id]
				ch.inbox <- &msg
				if !msg.More {
					ch.done <- ch
					close(ch.inbox)
					delete(peer.repCh, msg.Id)
				}
			}
		default:
			panic("bad msg type: " + msg.Type)
		}
	}
	peer.closeCh <- true
}

func (peer *Peer) Close() error {
	return peer.conn.Close()
}

func (peer *Peer) Call(method string, args interface{}, reply interface{}) error {
	ch := peer.Open(method)
	err := ch.Send(args, false)
	if err != nil {
		return err
	}
	if reply != nil {
		_, err = ch.Recv(reply)
		if err != nil {
			return err
		}
	}
	return (<-ch.done).err
}

func (peer *Peer) Open(service string) *Channel {
	ch := NewChannel(peer, TypeRequest, service)
	peer.counter = peer.counter + 1
	ch.id = peer.counter
	peer.repCh[ch.id] = ch
	return ch
}

type Channel struct {
	*Peer

	inbox  chan *Message
	done   chan *Channel
	ext    interface{}
	typ    string
	method string
	id     int
	err    error
}

func NewChannel(peer *Peer, typ string, method string) *Channel {
	return &Channel{
		Peer:   peer,
		inbox:  make(chan *Message, BacklogSize),
		done:   make(chan *Channel, 1), // buffered
		ext:    nil,
		typ:    typ,
		method: method,
		id:     0,
		err:    nil,
	}
}

func (ch *Channel) SetExt(ext interface{}) {
	ch.ext = ext
}

// TODO: SendError

// not convenient enough? we'll see
func (ch *Channel) SendLast(obj interface{}) error {
	return ch.Send(obj, false)
}

func (ch *Channel) Send(obj interface{}, more bool) error {
	msg := Message{
		Type:    ch.typ,
		Method:  ch.method,
		Payload: obj,
		More:    more,
		Id:      ch.id,
		Ext:     ch.ext,
	}
	frame, err := ch.rpc.codec.Encode(msg)
	if err != nil {
		return err
	}
	_, err = ch.conn.Write(frame)
	if ch.id == 0 {
		ch.done <- ch
		close(ch.inbox)
	}
	return err
}

func (ch *Channel) Recv(obj interface{}) (bool, error) {
	select {
	case msg, ok := <-ch.inbox:
		if !ok {
			return false, ch.err
		}
		payload := reflect.ValueOf(msg.Payload)
		reflect.ValueOf(obj).Elem().Set(payload)
		return msg.More, nil
	}
}

/*
// convenience method to read []byte object
func (ch *Channel) Read(p []byte) (n int, err error) {

}

// convenience method to write []byte object
func (ch *Channel) Write(p []byte) (n int, err error) {

}
*/
