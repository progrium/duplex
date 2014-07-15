package duplex

import (
	"reflect"
	"sync"

	"github.com/robxu9/duplex/bindings/go-duplex/dpx"
)

type Peer struct {
	P *dpx.Peer

	serviceLock sync.Mutex
	serviceMap  map[string]*service
	contextType reflect.Type
}

func NewPeer() *Peer {
	p := new(Peer)
	p.P = dpx.NewPeer()
	p.contextType = reflect.TypeOf("")
	p.serviceMap = make(map[string]*service)
	return p
}

func (p *Peer) Bind(addr string) error {
	return dpx.Bind(p.P, addr)
}

func (p *Peer) Connect(addr string) error {
	return dpx.Connect(p.P, addr)
}

func (p *Peer) Close() error {
	return dpx.Close(p.P)
}

func (p *Peer) Accept() (string, *Channel) {
	method, ch := dpx.Accept(p.P)
	if ch != nil {
		return method, &Channel{C: ch}
	}
	return "", nil
}

type Channel struct {
	C *dpx.Channel
}

func (c *Channel) Send(obj interface{}) error {
	return dpx.Send(c.C, obj)
}

func (c *Channel) SendLast(obj interface{}) error {
	return dpx.SendLast(c.C, obj)
}

func (c *Channel) SendErr(err string, last bool) error {
	return dpx.SendErr(c.C, err, last)
}

func (c *Channel) Receive(obj interface{}) error {
	return dpx.Receive(c.C, obj)
}

func (c *Channel) Close() error {
	return c.SendErr(dpx.CloseStreamErr, false)
}

var typeOfChannel = reflect.TypeOf(Channel{})

// RemoteError represents an error that has been returned from
// the remote side of the RPC connection.
type RemoteError string

func (e RemoteError) Error() string {
	return "remote: " + string(e)
}

type SendStream struct {
	channel *Channel
}

func (s *SendStream) Send(obj interface{}) error {
	return s.channel.Send(obj)
}

func (s *SendStream) SendLast(obj interface{}) error {
	return s.channel.SendLast(obj)
}

var typeOfSendStream = reflect.TypeOf(SendStream{})
