package duplex

import (
	"errors"
	"io"
	"log"
	"reflect"

	"github.com/robxu9/duplex/bindings/go-duplex/dpx"
)

type Call struct {
	channel *Channel
	peer    *Peer

	Method       string // The name of the service and method to call.
	Input        interface{}
	InputStream  *SendStream
	Output       interface{}
	OutputStream interface{} // chan *struct
	Error        error       // After completion, the error status.
	Done         chan *Call  // Strobes when call is complete

}

func (c *Call) Close() error {
	return c.channel.Close()
}

func (c *Call) done() {
	select {
	case c.Done <- c:
	default:
		log.Println("duplex: discarding extra Call completion")
		return
	}
	if c.OutputStream != nil {
		reflect.ValueOf(c.OutputStream).Close()
	}
}

func isChanOfPointers(c interface{}) bool {
	typ := reflect.TypeOf(c)
	// FIXME: check the direction of the channel, maybe?
	if typ.Kind() != reflect.Chan || typ.Elem().Kind() != reflect.Ptr {
		return false
	}
	return true
}

/*
	In effect, this method could be thought of as having these signatures:

		Open(method string, arg T1, reply *T2) (*Call, error)
		Open(method string, arg T1, replyStream chan *T2) (*Call, error)
		Open(method string, argStream *SendStream, replyStream chan *T2) (*Call, error)
		Open(method string, argStream *SendStream, reply *T2) (*Call, error)
*/
func (peer *Peer) Open(method string, input interface{}, output interface{}) (*Call, error) {
	channel := dpx.Open(peer.P, method)
	if channel == nil {
		return nil, errors.New("duplex: peer is closed")
	}
	call := new(Call)
	call.channel = &Channel{C: channel}
	call.peer = peer
	call.Method = method
	call.Done = make(chan *Call, 1) // buffered

	inType := reflect.TypeOf(input)
	if inType == reflect.PtrTo(typeOfSendStream) {
		call.InputStream = input.(*SendStream)
		call.InputStream.channel = call.channel
	} else {
		call.Input = input
		err := call.channel.SendLast(input)
		if err != nil {
			return nil, err
		}
	}

	if isChanOfPointers(output) {
		call.OutputStream = output
		go func() {
			defer call.done()
			for {
				// call.OutputStream is a chan *T2
				// we need to create a T2 and get a *T2 back
				value := reflect.New(reflect.TypeOf(call.OutputStream).Elem().Elem()).Interface()
				err := call.channel.Receive(value)
				if err != nil {
					if err != io.EOF {
						call.Error = RemoteError(err.Error())
					}
					return
				} else {
					if value != nil {
						// writing on the channel could block forever. For
						// instance, if a client calls 'close', this might block
						// forever.  the current suggestion is for the
						// client to drain the receiving channel in that case
						reflect.ValueOf(call.OutputStream).Send(reflect.ValueOf(value))
					} else {
						return
					}
				}
			}
		}()
	} else {
		call.Output = output
		go func() {
			defer call.done()
			err := call.channel.Receive(call.Output)
			if err != nil && err != io.EOF {
				// TODO handle non remote/parsing error
				call.Error = RemoteError(err.Error())
			}
		}()
	}
	return call, nil
}

// Call invokes the named function, waits for it to complete, and returns its error status.
func (peer *Peer) Call(method string, args interface{}, reply interface{}) error {
	call, err := peer.Open(method, args, reply)
	if err != nil {
		return err
	}
	return (<-call.Done).Error
}
