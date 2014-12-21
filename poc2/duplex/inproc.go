package duplex

import (
	"bytes"
	"errors"
	"net/url"
)

var inproc_listeners = make(map[string]*Peer)

// inproc connection

type inproc_peerConnection struct {
	identifier string
	name       string
	remote     *Peer
}

func (c *inproc_peerConnection) Disconnect() error {
	// TODO close all channels
	c.remote.Lock()
	delete(c.remote.conns, "inproc://"+c.identifier)
	c.remote.Unlock()
	return nil
}

func (c *inproc_peerConnection) Name() string {
	return c.name
}

func (c *inproc_peerConnection) Endpoint() string {
	return "inproc://" + c.identifier
}

func (c *inproc_peerConnection) Open(service string, headers []string) (Channel, error) {
	ch := &inproc_opened_ch{
		service:   service,
		headers:   headers,
		framesOut: make(chan []byte, 1024),
		framesIn:  make(chan []byte, 1024),
	}
	c.remote.incomingCh <- &inproc_accepted_ch{ch}
	return ch, nil
}

func newPeerConnection_inproc(peer *Peer, u *url.URL) (peerConnection, error) {
	identifier := u.Host
	remote, ok := inproc_listeners[identifier]
	if !ok {
		return nil, errors.New("no peer listening with this identifier: " + identifier)
	}
	remote.Lock()
	remote.conns["inproc://"+identifier] = &inproc_peerConnection{
		identifier: identifier,
		remote:     peer,
		name:       peer.GetOption(OptName),
	}
	remote.Unlock()
	return &inproc_peerConnection{
		identifier: identifier,
		remote:     remote,
		name:       remote.GetOption(OptName),
	}, nil
}

// inproc listener

type inproc_peerListener struct {
	identifier string
}

func (l *inproc_peerListener) Unbind() error {
	delete(inproc_listeners, l.identifier)
	return nil
}

func newPeerListener_inproc(peer *Peer, u *url.URL) (peerListener, error) {
	identifier := u.Host
	_, exists := inproc_listeners[identifier]
	if exists {
		return nil, errors.New("peer already listening with identifier: " + identifier)
	}
	inproc_listeners[identifier] = peer
	return &inproc_peerListener{identifier}, nil
}

// channels

type inproc_opened_ch struct {
	service   string
	headers   []string
	framesOut chan []byte
	framesIn  chan []byte
	bufferOut bytes.Buffer
	bufferIn  bytes.Buffer
}

func (c *inproc_opened_ch) ReadFrame() ([]byte, error) {
	return <-c.framesIn, nil
}

func (c *inproc_opened_ch) WriteFrame(frame []byte) error {
	c.framesOut <- frame
	return nil
}

func (c *inproc_opened_ch) Write(data []byte) (int, error) {
	return c.bufferOut.Write(data)
}

func (c *inproc_opened_ch) Read(data []byte) (int, error) {
	return c.bufferIn.Read(data)
}

func (c *inproc_opened_ch) CloseWrite() error {
	return nil // TODO
}

func (c *inproc_opened_ch) Close() error {
	return nil // TODO
}

func (c *inproc_opened_ch) Headers() []string {
	return c.headers
}

func (c *inproc_opened_ch) Service() string {
	return c.service
}

type inproc_accepted_ch struct {
	opened *inproc_opened_ch
}

func (c *inproc_accepted_ch) ReadFrame() ([]byte, error) {
	return <-c.opened.framesOut, nil
}

func (c *inproc_accepted_ch) WriteFrame(frame []byte) error {
	c.opened.framesIn <- frame
	return nil
}

func (c *inproc_accepted_ch) Write(data []byte) (int, error) {
	return c.opened.bufferIn.Write(data)
}

func (c *inproc_accepted_ch) Read(data []byte) (int, error) {
	return c.opened.bufferOut.Read(data)
}

func (c *inproc_accepted_ch) CloseWrite() error {
	return nil // TODO
}

func (c *inproc_accepted_ch) Close() error {
	return nil // TODO
}

func (c *inproc_accepted_ch) Headers() []string {
	return c.opened.headers
}

func (c *inproc_accepted_ch) Service() string {
	return c.opened.service
}
