package rpc

import (
	"io"
	"log"
	"reflect"
	"sync"

	"github.com/progrium/duplex/poc2/duplex"
)

type Peer struct {
	*duplex.Peer

	serviceLock sync.Mutex
	serviceMap  map[string]*service
	contextType reflect.Type
}

func NewPeer() *Peer {
	return &Peer{
		Peer:        duplex.NewPeer(),
		contextType: reflect.TypeOf(""),
		serviceMap:  make(map[string]*service),
	}
}

func (p *Peer) Accept() (duplex.ChannelMeta, *Channel) {
	meta, ch := p.Peer.Accept()
	if meta != nil {
		return meta, newChannel(ch)
	}
	return nil, nil
}

func (p *Peer) Open(peer, service string, headers []string) (*Channel, error) {
	ch, err := p.Peer.Open(peer, service, headers)
	return newChannel(ch), err
}

type Channel struct {
	duplex.Channel
	errorCh chan error
}

var typeOfChannel = reflect.TypeOf(Channel{})

func newChannel(ch duplex.Channel) *Channel {
	errorCh := make(chan error, 1024)
	go func() {
		for {
			frame, err := ch.ReadError()
			if err != nil {
				if err != io.EOF {
					log.Println("debug:", err)
				}
				return
			}
			errorCh <- RemoteError(frame)
		}
	}()
	return &Channel{Channel: ch, errorCh: errorCh}
}

func (ch *Channel) WriteObject(obj interface{}) error {
	frame, err := codecEncode(obj)
	if err != nil {
		return err
	}
	return ch.WriteFrame(frame)
}

func (ch *Channel) ReadObject(obj interface{}) error {
	select {
	case err := <-ch.errorCh:
		return err
	default:
		frame, err := ch.ReadFrame()
		if err != nil {
			return err
		}
		return codecDecode(frame, obj)
	}
}

type SendStream struct {
	channel *Channel
}

var typeOfSendStream = reflect.TypeOf(SendStream{})

func (s *SendStream) Send(obj interface{}) error {
	return s.channel.WriteObject(obj)
}

func (s *SendStream) SendLast(obj interface{}) error {
	err := s.channel.WriteObject(obj)
	if err != nil {
		return err
	}
	return s.channel.CloseWrite()
}

// RemoteError represents an error that has been returned from
// the remote side of the RPC connection.
type RemoteError string

func (e RemoteError) Error() string {
	return "remote: " + string(e)
}
