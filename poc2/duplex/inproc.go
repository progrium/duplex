package duplex

import (
	"bytes"
	"errors"
	"io"
	"net/url"
	"sync"
)

var inproc_listeners = make(map[string]*Peer)
var inproc_mutex sync.Mutex

// inproc connection

type inproc_peerConnection struct {
	sync.Mutex
	identifier string
	name       string
	remote     *Peer
	chans      []*inproc_opened_ch
}

func (c *inproc_peerConnection) Disconnect() error {
	c.Lock()
	defer c.Unlock()
	for _, ch := range c.chans {
		ch.Close()
	}
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
	c.Lock()
	defer c.Unlock()
	ch := &inproc_opened_ch{
		service:   service,
		headers:   headers,
		framesOut: make(chan []byte, 1024),
		framesIn:  make(chan []byte, 1024),
		errorsOut: make(chan []byte, 1024),
		errorsIn:  make(chan []byte, 1024),
	}
	c.chans = append(c.chans, ch)
	c.remote.incomingCh <- &inproc_accepted_ch{ch}
	return ch, nil
}

func newPeerConnection_inproc(peer *Peer, u *url.URL) (peerConnection, error) {
	inproc_mutex.Lock()
	defer inproc_mutex.Unlock()
	remote, ok := inproc_listeners[u.Host]
	if !ok {
		return nil, errors.New("no peer listening with this identifier: " + u.Host)
	}
	remote.Lock()
	remote.conns["inproc://"+peer.GetOption(OptName)] = &inproc_peerConnection{
		identifier: peer.GetOption(OptName),
		remote:     peer,
		name:       peer.GetOption(OptName),
		chans:      make([]*inproc_opened_ch, 0),
	}
	remote.Unlock()
	return &inproc_peerConnection{
		identifier: u.Host,
		remote:     remote,
		name:       remote.GetOption(OptName),
		chans:      make([]*inproc_opened_ch, 0),
	}, nil
}

// inproc listener

type inproc_peerListener struct {
	identifier string
}

func (l *inproc_peerListener) Unbind() error {
	inproc_mutex.Lock()
	defer inproc_mutex.Unlock()
	_, ok := inproc_listeners[l.identifier]
	if !ok {
		return errors.New("no peer listening with this identifier: " + l.identifier)
	}
	delete(inproc_listeners, l.identifier)
	return nil
}

func newPeerListener_inproc(peer *Peer, u *url.URL) (peerListener, error) {
	inproc_mutex.Lock()
	defer inproc_mutex.Unlock()
	_, exists := inproc_listeners[u.Host]
	if exists {
		return nil, errors.New("peer already listening with identifier: " + u.Host)
	}
	inproc_listeners[u.Host] = peer
	return &inproc_peerListener{u.Host}, nil
}

// channels

type inproc_opened_ch struct {
	sync.Mutex
	service   string
	headers   []string
	framesOut chan []byte
	framesIn  chan []byte
	errorsOut chan []byte
	errorsIn  chan []byte
	bufferOut bytes.Buffer
	bufferIn  bytes.Buffer
	eofOut    bool
	eofIn     bool
	closed    bool
}

func (c *inproc_opened_ch) ReadFrame() ([]byte, error) {
	frame, ok := <-c.framesIn
	if ok {
		return frame, nil
	}
	return nil, io.EOF
}

func (c *inproc_opened_ch) WriteFrame(frame []byte) error {
	c.Lock()
	defer c.Unlock()
	if c.closed || c.eofOut {
		return errors.New("channel closed")
	}
	c.framesOut <- frame
	return nil
}

func (c *inproc_opened_ch) ReadError() ([]byte, error) {
	err, ok := <-c.errorsIn
	if ok {
		return err, nil
	}
	return nil, io.EOF
}

func (c *inproc_opened_ch) WriteError(frame []byte) error {
	c.Lock()
	defer c.Unlock()
	if c.closed || c.eofOut {
		return errors.New("channel closed")
	}
	c.errorsOut <- frame
	return nil
}

func (c *inproc_opened_ch) Write(data []byte) (int, error) {
	c.Lock()
	defer c.Unlock()
	if c.closed || c.eofOut {
		return 0, errors.New("channel closed")
	}
	return c.bufferOut.Write(data)
}

func (c *inproc_opened_ch) Read(data []byte) (int, error) {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return 0, errors.New("channel closed")
	}
	return c.bufferIn.Read(data)
}

func (c *inproc_opened_ch) CloseWrite() error {
	c.Lock()
	defer c.Unlock()
	if c.closed || c.eofOut {
		return errors.New("channel already closed")
	}
	c.eofOut = true
	close(c.framesOut)
	return nil
}

func (c *inproc_opened_ch) Close() error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return errors.New("channel already closed")
	}
	if !c.eofOut {
		close(c.framesOut)
	}
	if !c.eofIn {
		close(c.framesIn)
	}
	close(c.errorsIn)
	close(c.errorsOut)
	c.closed = true
	return nil
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
	frame, ok := <-c.opened.framesOut
	if ok {
		return frame, nil
	}
	return nil, io.EOF
}

func (c *inproc_accepted_ch) WriteFrame(frame []byte) error {
	c.opened.Lock()
	defer c.opened.Unlock()
	if c.opened.closed || c.opened.eofIn {
		return errors.New("channel closed")
	}
	c.opened.framesIn <- frame
	return nil
}

func (c *inproc_accepted_ch) ReadError() ([]byte, error) {
	err, ok := <-c.opened.errorsOut
	if ok {
		return err, nil
	}
	return nil, io.EOF
}

func (c *inproc_accepted_ch) WriteError(frame []byte) error {
	c.opened.Lock()
	defer c.opened.Unlock()
	if c.opened.closed || c.opened.eofIn {
		return errors.New("channel closed")
	}
	c.opened.errorsIn <- frame
	return nil
}

func (c *inproc_accepted_ch) Write(data []byte) (int, error) {
	c.opened.Lock()
	defer c.opened.Unlock()
	if c.opened.closed || c.opened.eofIn {
		return 0, errors.New("channel closed")
	}
	return c.opened.bufferIn.Write(data)
}

func (c *inproc_accepted_ch) Read(data []byte) (int, error) {
	c.opened.Lock()
	defer c.opened.Unlock()
	if c.opened.closed {
		return 0, errors.New("channel closed")
	}
	return c.opened.bufferOut.Read(data)
}

func (c *inproc_accepted_ch) CloseWrite() error {
	c.opened.Lock()
	defer c.opened.Unlock()
	if c.opened.closed || c.opened.eofIn {
		return errors.New("channel already closed")
	}
	c.opened.eofIn = true
	close(c.opened.framesIn)
	return nil
}

func (c *inproc_accepted_ch) Close() error {
	return c.opened.Close()
}

func (c *inproc_accepted_ch) Headers() []string {
	return c.opened.headers
}

func (c *inproc_accepted_ch) Service() string {
	return c.opened.service
}
