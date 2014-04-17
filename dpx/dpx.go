package dpx

import (
	"bytes"
	"io"

	"github.com/ugorji/go/codec"
)

// This file contains the public API functions. They would
// look like dpx_peer, dpx_connect, etc when ported to C (libtask)

var mh codec.MsgpackHandle
var CloseStreamErr = "CloseStream"

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
	if channel.err != nil {
		return channel.err
	}
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
	if channel.incoming.Closed() {
		return nil
	}
	frame := channel.incoming.Dequeue()
	if channel.incoming.Draining() && frame == nil {
		channel.incoming.Close()
		// ok to remove since only affects incoming frames
		channel.conn.removeChannel(channel)
	}
	if frame != nil {
		return frame.(*Frame)
	}
	return nil
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

// blocks
func Receive(channel *Channel, obj interface{}) error {
	if channel.err != nil {
		return channel.err
	}
	return Decode(channel, ReceiveFrame(channel), obj)
}

func Decode(ch *Channel, frame *Frame, obj interface{}) error {
	if frame == nil {
		return io.EOF
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

// blocks
func Accept(peer *Peer) (string, *Channel) {
	ch := peer.accept()
	return ch.method, ch
}
