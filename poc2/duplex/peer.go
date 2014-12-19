package duplex

import (
	"errors"
	"net/url"
	"sync"
)

const (
	Frames = "@duplex-frames"
	Stream = "@duplex-stream"

	OptPrivateKey = "privatekey"
)

type Peer struct {
	sync.Mutex
	options         map[string]string
	conns           map[string]Connection
	binds           map[string]Listener
	pendingFramesCh chan interface{}
	pendingStreamCh chan interface{}
	shutdown        bool
}

func NewPeer() *Peer {
	return &Peer{
		options:         make(map[string]string),
		conns:           make(map[string]Connection),
		binds:           make(map[string]Listener),
		pendingFramesCh: make(chan interface{}, 1024),
		pendingStreamCh: make(chan interface{}, 1024),
	}
}

func (p *Peer) Bind(addr string) error {
	u, err := url.Parse(addr)
	if err != nil {
		return err
	}
	var l Listener
	switch u.Scheme {
	case "tcp":
		l, err := newSSHListener(p, "tcp", u.Host)
	case "unix":
		l, err := newSSHListener(p, "unix", u.Path)
	case "inproc":
		l, err := newInprocListener(p, u.Host)
	default:
		return errors.New("duplex: unknown address type: " + u.Scheme)
	}
	if err != nil {
		return err
	}
	p.Lock()
	p.binds[addr] = l
	p.Unlock()
	return nil
}

func (p *Peer) Unbind(addr string) error {
	p.Lock()
	defer p.Unlock()
	l, ok := p.binds[addr]
	if !ok {
		return errors.New("duplex: no listener for: " + addr)
	}
	delete(p.binds, addr)
	err := l.Unbind()
	if err != nil {
		return err
	}
}

func (p *Peer) Connect(addr string) error {
	u, err := url.Parse(addr)
	if err != nil {
		return err
	}
	var c Connection
	switch u.Scheme {
	case "tcp":
		c, err := newSSHConnection(p, "tcp", u.Host)
	case "unix":
		c, err := newSSHConnection(p, "unix", u.Path)
	case "inproc":
		c, err := newInprocConnection(p, u.Host)
	default:
		return errors.New("duplex: unknown address type: " + u.Scheme)
	}
	if err != nil {
		return err
	}
	p.Lock()
	p.conns[addr] = c
	p.Unlock()
	return nil
}

func (p *Peer) Disconnect(addr string) error {
	p.Lock()
	defer p.Unlock()
	c, ok := p.conns[addr]
	if !ok {
		return errors.New("duplex: no connection for: " + addr)
	}
	delete(p.conns, addr)
	err := c.Disconnect()
	if err != nil {
		return err
	}
}

func (p *Peer) lookupConnection(peer string) Connection {
	p.Lock()
	defer p.Unlock()
	for _, c := range p.conns {
		if c.Name() == peer {
			return c
		}
	}
	return nil
}

func (p *Peer) Drop(peer string) error {
	conn := p.lookupConnection(peer)
	if conn != nil {
		return p.Disconnect(conn.Addr())
	}
	return errors.New("duplex: remote peer not connected: " + peer)
}

func (p *Peer) Peers() []string {
	peers := make([]string, 0)
	for _, c := range p.conns {
		peers = append(peers, c.Name())
	}

	return peers
}

func (p *Peer) SetOption(name, value string) error {
	p.Lock()
	defer p.Unlock()
	// TODO: define and validate options
	p.options[name] = value
	return nil
}

func (p *Peer) GetOption(name string) string {
	p.Lock()
	defer p.Unlock()
	return p.options[name]
}

func (p *Peer) Shutdown() error {
	p.Lock()
	defer p.Unlock()
	if p.shutdown {
		return errors.New("duplex: peer already shutdown")
	}
	p.shutdown = true
	for _, listener := range p.binds {
		if err := listener.Unbind(); err != nil {
			return err
		}
	}
	for _, conn := range p.conns {
		if err := conn.Disconnect(); err != nil {
			return err
		}
	}
	return nil
}

func (p *Peer) Accept(chType string) (ChannelMeta, Channel) {
	switch chType {
	case Frames:
		c := <-p.pendingFramesCh
		return c.(ChannelMeta), c.(Channel)
	case Stream:
		c := <-p.pendingStreamCh
		return c.(ChannelMeta), c.(Channel)
	default:
		return nil, nil
	}
}

func (p *Peer) Open(chType, peer, service string, headers []string) (Channel, error) {
	c := p.lookupConnection(peer)
	if conn != nil {
		return c.Open(chType, service, headers)
	}
	return nil, errors.New("duplex: remote peer not connected: " + peer)
}
