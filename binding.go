package duplex

import (
	"errors"
	"log"
	"reflect"

	"./dpx"
)

// This is the high level API in Go that mostly adds language specific
// conveniences around the core Duplex API that would eventually be in C

type Peer struct {
	P *dpx.Peer
}

func NewPeer() *Peer {
	p := new(Peer)
	p.P = dpx.NewPeer()
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
	return method, &Channel{C: ch}
}

type Channel struct {
	C *dpx.Channel
}

func (c *Channel) Send(obj interface{}) error {
	return dpx.Send(c.C, obj)
}

func (c *Channel) Receive(obj interface{}) error {
	return dpx.Receive(c.C, obj)
}

// ServerError represents an error that has been returned from
// the remote side of the RPC connection.
type ServerError string

func (e ServerError) Error() string {
	return string(e)
}

type Stream struct {
	Send  chan<- interface{}
	Error <-chan error
}

type Call struct {
	ServiceMethod string      // The name of the service and method to call.
	Args          interface{} // The argument to the function (*struct).
	Reply         interface{} // The reply from the function (*struct for single, chan * struct for streaming).
	Error         error       // After completion, the error status.
	Done          chan *Call  // Strobes when call is complete (nil for streaming RPCs)
	Stream        bool        // True for a streaming RPC call, false otherwise

	channel *dpx.Channel
	peer    *Peer
}

func (c *Call) CloseStream() error {
	if !c.Stream {
		return errors.New("duplex: cannot close non-stream request")
	}

	// TODO send close stream control frame
	return nil
}

func (c *Call) done() {
	if c.Stream {
		// need to close the channel. Client won't be able to read any more.
		reflect.ValueOf(c.Reply).Close()
		return
	}

	select {
	case c.Done <- c:
		// ok
	default:
		// We don't want to block here.  It is the caller's responsibility to make
		// sure the channel has enough buffer space. See comment in Go().
		log.Println("duplex: discarding Call reply due to insufficient Done chan capacity")
	}
}

// Go invokes the function asynchronously.  It returns the Call structure representing
// the invocation.  The done channel will signal when the call is complete by returning
// the same Call object.  If done is nil, Go will allocate a new channel.
// If non-nil, done must be buffered or Go will deliberately crash.
func (peer *Peer) Go(serviceMethod string, args interface{}, reply interface{}, done chan *Call) *Call {
	call := new(Call)
	call.ServiceMethod = serviceMethod
	call.Args = args
	call.Reply = reply
	if done == nil {
		done = make(chan *Call, 10) // buffered.
	} else {
		// If caller passes done != nil, it must arrange that
		// done has enough buffer for the number of simultaneous
		// RPCs that will be using that channel.  If the channel
		// is totally unbuffered, it's best not to run at all.
		if cap(done) == 0 {
			log.Panic("duplex: done channel is unbuffered")
		}
	}
	call.Done = done
	call.peer = peer
	call.channel = dpx.Open(peer.P, call.ServiceMethod)
	dpx.Send(call.channel, call.Args)
	go func() {
		err := dpx.Receive(call.channel, call.Reply)
		if err != nil {
			// TODO handle non remote/parsing error
			call.Error = ServerError(err.Error())
		}
		call.done()
	}()
	return call
}

// Go invokes the streaming function asynchronously.  It returns the Call structure representing
// the invocation.
func (peer *Peer) StreamGo(serviceMethod string, args interface{}, replyStream interface{}) *Call {
	// first check the replyStream object is a stream of pointers to a data structure
	typ := reflect.TypeOf(replyStream)
	// FIXME: check the direction of the channel, maybe?
	if typ.Kind() != reflect.Chan || typ.Elem().Kind() != reflect.Ptr {
		log.Panic("duplex: replyStream is not a channel of pointers")
		return nil
	}

	call := new(Call)
	call.ServiceMethod = serviceMethod
	call.Args = args
	call.Reply = replyStream
	call.Stream = true
	call.peer = peer
	call.channel = dpx.Open(peer.P, call.ServiceMethod)
	dpx.Send(call.channel, call.Args)
	go func() {
		defer call.done()
		for {
			// call.Reply is a chan *T2
			// we need to create a T2 and get a *T2 back
			value := reflect.New(reflect.TypeOf(call.Reply).Elem().Elem()).Interface()
			err := dpx.Receive(call.channel, value)
			if err != nil {
				call.Error = ServerError(err.Error())
				return
			} else {
				if value != nil {
					// writing on the channel could block forever. For
					// instance, if a client calls 'close', this might block
					// forever.  the current suggestion is for the
					// client to drain the receiving channel in that case
					reflect.ValueOf(call.Reply).Send(reflect.ValueOf(value))
				} else {
					return
				}
			}
		}
	}()
	return call
}

// Call invokes the named function, waits for it to complete, and returns its error status.
func (peer *Peer) Call(serviceMethod string, args interface{}, reply interface{}) error {
	call := <-peer.Go(serviceMethod, args, reply, make(chan *Call, 1)).Done
	return call.Error
}
