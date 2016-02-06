package duplex

import (
	"errors"
	"log"
	"math"
	"math/rand"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/pborman/uuid"
)

const (
	OptPrivateKey           = iota // string (filepath)
	OptAuthorizedKeys              // string (filepath)
	OptName                        // string
	OptReconnectInterval           // int (milliseconds)
	OptReconnectIntervalMax        // int (milliseconds)

	retryFactor = math.E        // 2.72ish, math.Phi could also be good
	retryJitter = 0.11962656472 // molar Planck constant times c, joule meter/mole
)

var debugMode = true

func debug(o ...interface{}) {
	if debugMode {
		log.Println("debug:", o)
	}
}

type Peer struct {
	sync.Mutex
	options    map[int]interface{}
	conns      map[string]peerConnection
	binds      map[string]peerListener
	incomingCh chan interface{}
	shutdown   bool
	rrIndex    int
}

type peerConnFactory func(peer *Peer, endpoint *url.URL) (peerConnection, error)

func NewPeer() *Peer {
	return &Peer{
		options: map[int]interface{}{
			OptPrivateKey:           "~/.ssh/id_rsa",
			OptAuthorizedKeys:       "~/.ssh/authorized_keys",
			OptName:                 uuid.New(),
			OptReconnectInterval:    100,
			OptReconnectIntervalMax: 0,
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
	case "tcp", "unix":
		l, err = newPeerListener_ssh(p, u)
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

func (p *Peer) retryConnect(fn peerConnFactory, endpoint *url.URL) (peerConnection, error) {
	if p.GetOption(OptReconnectInterval).(int) == -1 {
		// no retry
		return fn(p, endpoint)
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	delay := float64(p.GetOption(OptReconnectInterval).(int))
	if p.GetOption(OptReconnectIntervalMax).(int) == 0 {
		// constant (but randomized) retry, no backoff
		for {
			c, err := fn(p, endpoint)
			if err == nil {
				return c, err
			}
			debug(err)
			time.Sleep(time.Duration(delay) * time.Millisecond)
			delay = r.NormFloat64()*(delay*retryJitter) + delay
		}
	}
	// backoff algorithm copied from Twisted's ReconnectingClientFactory
	// http://twistedmatrix.com/trac/browser/tags/releases/twisted-8.2.0/twisted/internet/protocol.py#L198
	delayMax := float64(p.GetOption(OptReconnectIntervalMax).(int))
	for {
		c, err := fn(p, endpoint)
		if err == nil {
			return c, err
		}
		time.Sleep(time.Duration(delay) * time.Millisecond)
		delay = math.Min(delay*retryFactor, delayMax)
		delay = r.NormFloat64()*(delay*retryJitter) + delay
	}
}

func (p *Peer) Connect(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil {
		return err
	}
	var c peerConnection
	switch u.Scheme {
	case "tcp", "unix":
		c, err = p.retryConnect(newPeerConnection_ssh, u)
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

func (p *Peer) NextPeer() string {
	peers := p.Peers()
	if len(peers) == 0 {
		return ""
	}
	p.Lock()
	p.rrIndex += 1
	p.Unlock()
	return peers[p.rrIndex%len(peers)]
}

func (p *Peer) SetOption(option int, value interface{}) error {
	p.Lock()
	defer p.Unlock()
	// TODO: define and validate options
	p.options[option] = value
	return nil
}

func (p *Peer) GetOption(option int) interface{} {
	p.Lock()
	defer p.Unlock()
	return p.options[option]
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
	if peer == "" {
		return nil, errors.New("duplex: empty peer for open")
	}
	c := p.lookupConnection(peer)
	if c != nil {
		return c.Open(service, headers)
	}
	return nil, errors.New("duplex: remote peer not connected: " + peer)
}
