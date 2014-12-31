package duplex

import (
	"bytes"
	"errors"
	"io"
	"net/url"
	"sync"
)

var inproc_listeners = struct {
	sync.Mutex
	m map[string]*Peer
}{
	m: make(map[string]*Peer),
}

// inproc connection

type inproc_peerConnection struct {
	sync.Mutex
	identifier string
	remote     *Peer
	local      *Peer
	chans      []*inproc_opened_ch
}

func (pc *inproc_peerConnection) Disconnect() error {
	pc.Lock()
	defer pc.Unlock()
	for _, ch := range pc.chans {
		ch.Close()
	}
	pc.remote.Lock()
	delete(pc.remote.conns, "inproc://"+pc.identifier)
	pc.remote.Unlock()
	return nil
}

func (pc *inproc_peerConnection) Name() string {
	return pc.remote.GetOption(OptName).(string)
}

func (pc *inproc_peerConnection) Endpoint() string {
	return "inproc://" + pc.identifier
}

func (pc *inproc_peerConnection) Open(service string, headers []string) (Channel, error) {
	ch := inproc_newChannel(pc, service, headers)
	pc.Lock()
	pc.chans = append(pc.chans, ch)
	pc.Unlock()
	pc.remote.incomingCh <- &inproc_accepted_ch{ch}
	return ch, nil
}

func newPeerConnection_inproc(peer *Peer, u *url.URL) (peerConnection, error) {
	inproc_listeners.Lock()
	defer inproc_listeners.Unlock()
	remote, ok := inproc_listeners.m[u.Host]
	if !ok {
		return nil, errors.New("no peer listening with this identifier: " + u.Host)
	}
	remote.Lock()
	remote.conns["inproc://"+peer.GetOption(OptName).(string)] = &inproc_peerConnection{
		identifier: peer.GetOption(OptName).(string),
		remote:     peer,
		local:      remote,
		chans:      make([]*inproc_opened_ch, 0),
	}
	remote.Unlock()
	return &inproc_peerConnection{
		identifier: u.Host,
		remote:     remote,
		local:      peer,
		chans:      make([]*inproc_opened_ch, 0),
	}, nil
}

// inproc listener

type inproc_peerListener struct {
	identifier string
}

func (pl *inproc_peerListener) Unbind() error {
	inproc_listeners.Lock()
	defer inproc_listeners.Unlock()
	_, ok := inproc_listeners.m[pl.identifier]
	if !ok {
		return errors.New("no peer listening with this identifier: " + pl.identifier)
	}
	delete(inproc_listeners.m, pl.identifier)
	return nil
}

func newPeerListener_inproc(peer *Peer, u *url.URL) (peerListener, error) {
	inproc_listeners.Lock()
	defer inproc_listeners.Unlock()
	_, exists := inproc_listeners.m[u.Host]
	if exists {
		return nil, errors.New("peer already listening with identifier: " + u.Host)
	}
	inproc_listeners.m[u.Host] = peer
	return &inproc_peerListener{u.Host}, nil
}

// channels

type inproc_opened_ch struct {
	sync.Mutex
	service     string
	headers     []string
	framesOut   chan []byte
	framesIn    chan []byte
	errorsOut   chan []byte
	errorsIn    chan []byte
	bufferOut   bytes.Buffer
	bufferIn    bytes.Buffer
	eofOut      bool
	eofIn       bool
	closed      bool
	localPeer   string
	remotePeer  string
	attachedIn  chan interface{}
	attachedOut chan interface{}
	peerConn    *inproc_peerConnection
}

func inproc_newChannel(pc *inproc_peerConnection, service string, headers []string) *inproc_opened_ch {
	return &inproc_opened_ch{
		peerConn:    pc,
		service:     service,
		headers:     headers,
		framesOut:   make(chan []byte, 1024),
		framesIn:    make(chan []byte, 1024),
		errorsOut:   make(chan []byte, 1024),
		errorsIn:    make(chan []byte, 1024),
		attachedIn:  make(chan interface{}, 1024),
		attachedOut: make(chan interface{}, 1024),
		localPeer:   pc.local.GetOption(OptName).(string),
		remotePeer:  pc.remote.GetOption(OptName).(string),
	}
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
	close(c.attachedIn)
	close(c.attachedOut)
	c.closed = true
	return nil
}

func (c *inproc_opened_ch) Headers() []string {
	return c.headers
}

func (c *inproc_opened_ch) Service() string {
	return c.service
}

func (c *inproc_opened_ch) Meta() ChannelMeta {
	return c
}

func (c *inproc_opened_ch) LocalPeer() string {
	return c.localPeer
}

func (c *inproc_opened_ch) RemotePeer() string {
	return c.remotePeer
}

func (c *inproc_opened_ch) Join(rwc io.ReadWriteCloser) {
	joinChannel(c, rwc)
}

func (c *inproc_opened_ch) Accept() (ChannelMeta, Channel) {
	ch := <-c.attachedIn
	if ch != nil {
		return ch.(ChannelMeta), ch.(Channel)
	}
	return nil, nil
}

func (c *inproc_opened_ch) Open(service string, headers []string) (Channel, error) {
	ch := inproc_newChannel(c.peerConn, service, headers)
	c.peerConn.Lock()
	c.peerConn.chans = append(c.peerConn.chans, ch)
	c.peerConn.Unlock()
	c.attachedOut <- &inproc_accepted_ch{ch}
	return ch, nil
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

func (c *inproc_accepted_ch) Meta() ChannelMeta {
	return c.opened
}

func (c *inproc_accepted_ch) LocalPeer() string {
	return c.opened.remotePeer
}

func (c *inproc_accepted_ch) RemotePeer() string {
	return c.opened.localPeer
}

func (c *inproc_accepted_ch) Join(rwc io.ReadWriteCloser) {
	joinChannel(c, rwc)
}

func (c *inproc_accepted_ch) Accept() (ChannelMeta, Channel) {
	ch := <-c.opened.attachedOut
	if ch != nil {
		return ch.(ChannelMeta), ch.(Channel)
	}
	return nil, nil
}

func (c *inproc_accepted_ch) Open(service string, headers []string) (Channel, error) {
	ch := inproc_newChannel(c.opened.peerConn, service, headers)
	c.opened.peerConn.Lock()
	c.opened.peerConn.chans = append(c.opened.peerConn.chans, ch)
	c.opened.peerConn.Unlock()
	c.opened.attachedIn <- &inproc_accepted_ch{ch}
	return ch, nil
}
