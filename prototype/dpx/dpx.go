package dpx

import (
	"bytes"
	"errors"
	"io"
	"log"

	"github.com/ugorji/go/codec"
)

// This file contains the public API functions. They would
// look like dpx_peer, dpx_connect, etc when ported to C (libtask)

var CloseStreamErr = "CloseStream"

var mh codec.MsgpackHandle
var debugMode = true

func debug(v ...interface{}) {
	if debugMode {
		log.Println(v...)
	}
}

// Peer operations

func NewPeer() *Peer {
	return newPeer()
}

func Connect(peer *Peer, addr string) error {
	return peer.Connect(addr)
}

func Bind(peer *Peer, addr string) error {
	return peer.Bind(addr)
}

func Close(peer *Peer) error {
	return peer.Close()
}

func Codec(peer *Peer, name string, codec interface{}) error {
	return nil // TODO
}

// Channel operations

func NewFrame(channel *Channel) *Frame {
	return newFrame(channel)
}

func SendFrame(channel *Channel, frame *Frame) error {
	return channel.SendFrame(frame)
}

// blocks
func ReceiveFrame(channel *Channel) *Frame {
	return channel.ReceiveFrame()
}

func Send(channel *Channel, data interface{}) error {
	frame := newFrame(channel)
	Encode(channel, frame, data)
	return SendFrame(channel, frame)
}

func SendLast(channel *Channel, data interface{}) error {
	frame := newFrame(channel)
	if data != nil {
		Encode(channel, frame, data)
	}
	frame.Last = true
	return SendFrame(channel, frame)
}

func SendErr(channel *Channel, err string, last bool) error {
	frame := newFrame(channel)
	frame.Error = err
	frame.Last = last
	return SendFrame(channel, frame)
}

// blocks
func Receive(channel *Channel, obj interface{}) error {
	if err := channel.Error(); err != nil {
		return err
	}
	return Decode(channel, ReceiveFrame(channel), obj)
}

func Decode(ch *Channel, frame *Frame, obj interface{}) error {
	if frame == nil {
		return io.EOF
	}
	if frame.Error != "" {
		return errors.New(frame.Error)
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
	return peer.Open(method)
}

func OpenWith(peer *Peer, uuid, method string) (*Channel, error) {
	return peer.OpenWith(uuid, method)
}

// Server operations

// blocks
func Accept(peer *Peer) (string, *Channel) {
	ch := peer.Accept()
	if ch != nil {
		return ch.Method, ch
	}
	return "", nil
}
