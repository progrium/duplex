package rpc

import (
	"errors"
	"io"
	"log"
	"reflect"
)

type Call struct {
	*Channel

	Method       string // The name of the service and method to call.
	Input        interface{}
	InputStream  *SendStream
	Output       interface{}
	OutputStream interface{} // chan *struct
	Error        error       // After completion, the error status.
	Done         chan *Call  // Strobes when call is complete

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

/*
	In effect, this method could be thought of as having these signatures:

		OpenCall(peer, method string, arg T1, reply *T2) (*Call, error)
		OpenCall(peer, method string, arg T1, replyStream chan *T2) (*Call, error)
		OpenCall(peer, method string, argStream *SendStream, reply *T2) (*Call, error)
		OpenCall(peer, method string, argStream *SendStream, replyStream chan *T2) (*Call, error)
*/
func (p *Peer) OpenCall(peer, method string, input interface{}, output interface{}) (*Call, error) {
	channel, err := p.Open(peer, method, nil)
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, errors.New("duplex: peer is closed")
	}
	call := new(Call)
	call.Channel = channel
	call.Method = method
	call.Done = make(chan *Call, 1) // buffered

	inType := reflect.TypeOf(input)
	if inType == reflect.PtrTo(typeOfSendStream) {
		call.InputStream = input.(*SendStream)
		call.InputStream.channel = call.Channel
	} else {
		call.Input = input
		err := call.WriteObject(input)
		if err != nil {
			return nil, err
		}
		err = call.CloseWrite()
		if err != nil {
			return nil, err
		}
	}

	typ := reflect.TypeOf(output)
	// FIXME: check the direction of the channel, maybe?
	if typ.Kind() != reflect.Chan || typ.Elem().Kind() != reflect.Ptr {
		// output is not channel of pointers
		call.Output = output
		go func() {
			defer call.done()
			err := call.ReadObject(call.Output)
			if err != nil && err != io.EOF {
				// TODO handle non remote/parsing error
				call.Error = RemoteError(err.Error())
			}
		}()
	} else {
		// output is channel of pointers
		call.OutputStream = output
		go func() {
			defer call.done()
			for {
				// call.OutputStream is a chan *T2
				// we need to create a T2 and get a *T2 back
				value := reflect.New(reflect.TypeOf(call.OutputStream).Elem().Elem()).Interface()
				err := call.ReadObject(value)
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
	}
	return call, nil
}

// Call invokes the named function, waits for it to complete, and returns its error status.
// It also uses NextPeer to round robin request channels across connections.
func (peer *Peer) Call(method string, args interface{}, reply interface{}) error {
	call, err := peer.OpenCall(peer.NextPeer(), method, args, reply)
	if err != nil {
		return err
	}
	return (<-call.Done).Error
}
