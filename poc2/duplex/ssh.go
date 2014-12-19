package duplex

import (
	"bytes"
	"errors"
	"io/ioutil"
	"log"
	"net"

	"github.com/progrium/crypto/ssh"
)

func newSSHListener(typ, addr string, peer *Peer) (Listener, error) {
	pem, err := ioutil.ReadFile(peer.GetOption(OptPrivateKey))
	if err != nil {
		return nil, err
	}
	pk, err := ssh.ParsePrivateKey(pem)
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
				peer.Unbind(typ + "://" + addr)
				return
			}
			go handleSSHConnection(conn, config)
		}
	}()
	return &sshListener{listener}, nil
}

type sshListener struct {
	net.Listener
}

func (l *sshListener) Unbind() error {
	return l.Close()
}

func handleSSHConn(conn net.Conn, config *ssh.ServerConfig) {
	defer conn.Close()
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		log.Println("duplex:ssh: failed to handshake:", err)
		return
	}
	go ssh.DiscardRequests(reqs)
	for ch := range chans {
		log.Println(sshConn.User(), ch.ChannelType())
		/*if ch.ChannelType() == "frames" {
			go handleFrameChannel(ch)
		}*/
	}
}

func newSSHConnection(typ, addr string, peer *Peer) (Connection, error) {
	pem, err := ioutil.ReadFile(peer.GetOption(OptPrivateKey))
	if err != nil {
		return nil, err
	}
	pk, err := ssh.ParsePrivateKey(pem)
	if err != nil {
		return nil, err
	}
	config := &ssh.ClientConfig{
		User: "jeff",
		Auth: []ssh.AuthMethod{ssh.PublicKeys(pk)},
	}
	client, err := ssh.Dial(typ, addr, config)
	if err != nil {
		return nil, err
	}
	return &sshConnection{
		addr:   typ + "://" + addr,
		name:   "foobar",
		client: client,
	}, nil
}

type sshConnection struct {
	addr   string
	name   string
	client *ssh.Client
}

func (c *sshConnection) Disconnect() error {
	return c.client.Close()
}

func (c *sshConnection) Name() string {
	return c.name
}

func (c *sshConnection) Addr() string {
	return c.addr
}

func (c *sshConnection) Open(chType, service string, headers []string) (Channel, error) {
	return nil, errors.New("not implemented")
}
