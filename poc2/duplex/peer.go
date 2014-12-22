package duplex

import (
	"errors"
	"net/url"
	"sort"
	"sync"

	"code.google.com/p/go-uuid/uuid"
)

const (
	OptPrivateKey     = "privatekey"
	OptAuthorizedKeys = "authorizedkeys"
	OptName           = "name"
)

type Peer struct {
	sync.Mutex
	options    map[string]string
	conns      map[string]peerConnection
	binds      map[string]peerListener
	incomingCh chan interface{}
	shutdown   bool
	rrIndex    int
}

func NewPeer() *Peer {
	return &Peer{
		options: map[string]string{
			OptPrivateKey:     "~/.ssh/id_rsa",
			OptAuthorizedKeys: "~/.ssh/authorized_keys",
			OptName:           uuid.New(),
		},
		conns:      make(map[string]peerConnection),
		binds:      make(map[string]peerListener),
		incomingCh: make(chan interface{}, 1024),
	}
}

func (p *Peer) Bind(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil {
		return err
	}
	var l peerListener
	switch u.Scheme {
	case "tcp":
		l, err = newPeerListener_ssh(p, u)
	case "unix":
		l, err = newPeerListener_ssh(p, u)
	case "inproc":
		l, err = newPeerListener_inproc(p, u)
	default:
		return errors.New("duplex: unknown endpoint type: " + u.Scheme)
	}
	if err != nil {
		return err
	}
	p.Lock()
	p.binds[endpoint] = l
	p.Unlock()
	return nil
}

func (p *Peer) Unbind(endpoint string) error {
	p.Lock()
	defer p.Unlock()
	l, ok := p.binds[endpoint]
	if !ok {
		return errors.New("duplex: no listener for: " + endpoint)
	}
	delete(p.binds, endpoint)
	err := l.Unbind()
	if err != nil {
		return err
	}
	return nil
}

func (p *Peer) Connect(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil {
		return err
	}
	var c peerConnection
	switch u.Scheme {
	case "tcp":
		c, err = newPeerConnection_ssh(p, u)
	case "unix":
		c, err = newPeerConnection_ssh(p, u)
	case "inproc":
		c, err = newPeerConnection_inproc(p, u)
	default:
		return errors.New("duplex: unknown endpoint type: " + u.Scheme)
	}
	if err != nil {
		return err
	}
	p.Lock()
	p.conns[endpoint] = c
	p.Unlock()
	return nil
}

func (p *Peer) Disconnect(endpoint string) error {
	p.Lock()
	defer p.Unlock()
	c, ok := p.conns[endpoint]
	if !ok {
		return errors.New("duplex: no connection for: " + endpoint)
	}
	delete(p.conns, endpoint)
	err := c.Disconnect()
	if err != nil {
		return err
	}
	return nil
}

func (p *Peer) lookupConnection(peer string) peerConnection {
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
		return p.Disconnect(conn.Endpoint())
	}
	return errors.New("duplex: remote peer not connected: " + peer)
}

func (p *Peer) Peers() []string {
	p.Lock()
	defer p.Unlock()
	peers := make([]string, 0)
	for _, c := range p.conns {
		peers = append(peers, c.Name())
	}
	sort.Strings(peers)
	return peers
}

func (p *Peer) NextPeer() (string, error) {
	peers := p.Peers()
	if len(peers) == 0 {
		return "", errors.New("no peers connected")
	}
	p.Lock()
	p.rrIndex += 1
	p.Unlock()
	return peers[p.rrIndex%len(peers)], nil
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
	close(p.incomingCh)
	return nil
}

func (p *Peer) Accept() (ChannelMeta, Channel) {
	c := <-p.incomingCh
	if c != nil {
		return c.(ChannelMeta), c.(Channel)
	}
	return nil, nil
}

func (p *Peer) Open(peer, service string, headers []string) (Channel, error) {
	c := p.lookupConnection(peer)
	if c != nil {
		return c.Open(service, headers)
	}
	return nil, errors.New("duplex: remote peer not connected: " + peer)
}
