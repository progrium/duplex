package duplex

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io/ioutil"
	"log"
	"net"
	"os/user"
	"strings"

	"github.com/progrium/crypto/ssh"
)

func expandPath(path string) string {
	if path[:2] == "~/" {
		usr, _ := user.Current()
		return strings.Replace(path, "~", usr.HomeDir, 1)
	}
	return path
}

func loadPrivateKey(path string) (ssh.Signer, error) {
	pem, err := ioutil.ReadFile(expandPath(path))
	if err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey(pem)
}

// channels

type channelMeta struct {
	Service string
	Headers []string
}

type greetingPayload struct {
	Name string
}

func appendU32(buf []byte, n uint32) []byte {
	return append(buf, byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
}

type channel_ssh struct {
	ssh.Channel
	channelMeta
}

func (c *channel_ssh) ReadFrame() ([]byte, error) {
	bytes := make([]byte, 4)
	_, err := c.Read(bytes)
	if err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(bytes)
	frame := make([]byte, length)
	_, err = c.Read(frame)
	// handle errors based on written bytes
	if err != nil {
		return nil, err
	}
	return frame, nil
}

func (c *channel_ssh) WriteFrame(frame []byte) error {
	var buffer []byte
	buffer = appendU32(buffer, uint32(len(frame)))
	buffer = append(buffer, frame...)
	_, err := c.Write(buffer)
	return err
}

func (c *channel_ssh) Headers() []string {
	return c.channelMeta.Headers
}

func (c *channel_ssh) Service() string {
	return c.channelMeta.Service
}

// server

type sshListener struct {
	net.Listener
}

func (l *sshListener) Unbind() error {
	return l.Close()
}

func newPeerListener_ssh(peer *Peer, typ, addr string) (peerListener, error) {
	pk, err := loadPrivateKey(peer.GetOption(OptPrivateKey))
	if err != nil {
		return nil, err
	}
	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if bytes.Equal(key.Marshal(), pk.PublicKey().Marshal()) {
				return &ssh.Permissions{}, nil
			}
			return nil, errors.New("unauthorized")
		},
	}
	config.AddHostKey(pk)

	listener, err := net.Listen(typ, addr)
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Println("debug: unbinding")
				peer.Unbind(typ + "://" + addr)
				return
			}
			go handleSSHConn(conn, config, peer)
		}
	}()
	return &sshListener{listener}, nil
}

func handleSSHConn(conn net.Conn, config *ssh.ServerConfig, peer *Peer) {
	defer conn.Close()
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		log.Println("debug: failed to handshake:", err)
		return
	}
	go ssh.DiscardRequests(reqs)
	peer.Lock()
	peer.conns[conn.RemoteAddr().String()] = &sshConnection{
		addr: conn.RemoteAddr().Network() + "://" + conn.RemoteAddr().String(),
		name: sshConn.User(),
		conn: sshConn,
	}
	peer.Unlock()
	ok, _, err := sshConn.SendRequest("@duplex-greeting", true,
		ssh.Marshal(&greetingPayload{peer.GetOption(OptName)}))
	if err != nil || !ok {
		log.Println("debug: failed to greet:", err)
		return
	}
	for ch := range chans {
		switch ch.ChannelType() {
		case "@duplex":
			go handleSSHChannel(ch, peer)
		}
	}
}

func handleSSHChannel(newChan ssh.NewChannel, peer *Peer) {
	var meta channelMeta
	err := ssh.Unmarshal(newChan.ExtraData(), &meta)
	if err != nil {
		log.Println("frame parse:", err)
		return
	}
	if meta.Service == "" {
		newChan.Reject(ssh.UnknownChannelType, "empty service")
		return
	}
	sshCh, reqs, err := newChan.Accept()
	if err != nil {
		log.Println("accept error:", err)
		return
	}
	go ssh.DiscardRequests(reqs)
	peer.incomingCh <- &channel_ssh{sshCh, meta}
}

// client

func newPeerConnection_ssh(peer *Peer, network, addr string) (peerConnection, error) {
	pk, err := loadPrivateKey(peer.GetOption(OptPrivateKey))
	if err != nil {
		return nil, err
	}
	config := &ssh.ClientConfig{
		User: peer.GetOption(OptName),
		Auth: []ssh.AuthMethod{ssh.PublicKeys(pk)},
	}
	netConn, err := net.Dial(network, addr)
	if err != nil {
		return nil, err
	}
	conn, chans, reqs, err := ssh.NewClientConn(netConn, addr, config)
	if err != nil {
		return nil, err
	}
	go func() {
		for ch := range chans {
			log.Println("client", ch.ChannelType())
			switch ch.ChannelType() {
			case "@duplex":
				go handleSSHChannel(ch, peer)
			}
		}
	}()
	nameCh := make(chan string)
	go func() {
		for r := range reqs {
			switch r.Type {
			case "@duplex-greeting":
				var greeting greetingPayload
				err := ssh.Unmarshal(r.Payload, &greeting)
				if err != nil {
					continue
				}
				nameCh <- greeting.Name
				r.Reply(true, nil)
			default:
				// This handles keepalive messages and matches
				// the behaviour of OpenSSH.
				r.Reply(false, nil)
			}
		}
	}()
	// todo: timeout nameCh
	return &sshConnection{
		addr: network + "://" + addr,
		name: <-nameCh,
		conn: conn,
	}, nil
}

type sshConnection struct {
	addr string
	name string
	conn ssh.Conn
}

func (c *sshConnection) Disconnect() error {
	return c.conn.Close()
}

func (c *sshConnection) Name() string {
	return c.name
}

func (c *sshConnection) Addr() string {
	return c.addr
}

func (c *sshConnection) Open(service string, headers []string) (Channel, error) {
	meta := channelMeta{
		Service: service,
		Headers: headers,
	}
	ch, reqs, err := c.conn.OpenChannel("@duplex", ssh.Marshal(meta))
	if err != nil {
		return nil, err
	}
	go ssh.DiscardRequests(reqs)
	return &channel_ssh{ch, meta}, nil
}
