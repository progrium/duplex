package duplex

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"os"
	"os/user"
	"strings"

	"github.com/progrium/crypto/ssh"
)

// ssh keys

func loadPrivateKey(path string) (ssh.Signer, error) {
	if path[:2] == "~/" {
		usr, _ := user.Current()
		path = strings.Replace(path, "~", usr.HomeDir, 1)
	}
	pem, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey(pem)
}

// ssh structs

type ssh_greetingPayload struct {
	Name string
}

type ssh_channelData struct {
	Service string
	Headers []string
}

// ssh listener

type ssh_peerListener struct {
	net.Listener
	path string
}

func (l *ssh_peerListener) Unbind() error {
	if l.path != "" {
		os.Remove(l.path)
	}
	return l.Close()
}

// ssh connection

type ssh_peerConnection struct {
	endpoint string
	name     string
	conn     ssh.Conn
}

func (c *ssh_peerConnection) Disconnect() error {
	return c.conn.Close()
}

func (c *ssh_peerConnection) Name() string {
	return c.name
}

func (c *ssh_peerConnection) Endpoint() string {
	return c.endpoint
}

func (c *ssh_peerConnection) Open(service string, headers []string) (Channel, error) {
	meta := ssh_channelData{
		Service: service,
		Headers: headers,
	}
	ch, reqs, err := c.conn.OpenChannel("@duplex", ssh.Marshal(meta))
	if err != nil {
		return nil, err
	}
	go ssh.DiscardRequests(reqs)
	return &ssh_channel{ch, meta}, nil
}

// ssh server

func newPeerListener_ssh(peer *Peer, u *url.URL) (peerListener, error) {
	pk, err := loadPrivateKey(peer.GetOption(OptPrivateKey).(string))
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
	var listener net.Listener
	if u.Scheme == "unix" {
		os.Remove(u.Path)
		listener, err = net.Listen(u.Scheme, u.Path)
	} else {
		listener, err = net.Listen(u.Scheme, u.Host)
	}
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				peer.Unbind(u.String())
				return
			}
			go ssh_handleConn(conn, config, peer)
		}
	}()
	if u.Scheme == "unix" {
		return &ssh_peerListener{listener, u.Path}, nil
	} else {
		return &ssh_peerListener{listener, ""}, nil
	}
}

func ssh_handleConn(conn net.Conn, config *ssh.ServerConfig, peer *Peer) {
	defer conn.Close()
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		log.Println("debug: failed to handshake:", err)
		return
	}
	endpoint := conn.RemoteAddr().Network() + "://" + conn.RemoteAddr().String()
	if conn.RemoteAddr().Network() == "unix" && conn.RemoteAddr().String() == "" {
		// is it normal to not have a remote address with unix sockets?
		// because we need a unique endpoint for peer.conns, we use the
		// peer name when we don't have a remote address.
		endpoint = conn.RemoteAddr().Network() + "://" + sshConn.User()
	}
	peer.Lock()
	peer.conns[endpoint] = &ssh_peerConnection{
		endpoint: endpoint,
		name:     sshConn.User(),
		conn:     sshConn,
	}
	peer.Unlock()
	/*go func() {
		sshConn.Wait()
		log.Println(peer.Disconnect(endpoint))
	}()*/
	go ssh.DiscardRequests(reqs)
	ok, _, err := sshConn.SendRequest("@duplex-greeting", true,
		ssh.Marshal(&ssh_greetingPayload{peer.GetOption(OptName).(string)}))
	if err != nil || !ok {
		if err != io.EOF {
			log.Println("debug: failed to greet:", err)
		}
		return
	}
	ssh_acceptChannels(chans, peer)
}

// ssh client

func newPeerConnection_ssh(peer *Peer, u *url.URL) (peerConnection, error) {
	pk, err := loadPrivateKey(peer.GetOption(OptPrivateKey).(string))
	if err != nil {
		return nil, err
	}
	config := &ssh.ClientConfig{
		User: peer.GetOption(OptName).(string),
		Auth: []ssh.AuthMethod{ssh.PublicKeys(pk)},
	}
	var addr string
	if u.Scheme == "unix" {
		addr = u.Path
	} else {
		addr = u.Host
	}
	netConn, err := net.Dial(u.Scheme, addr)
	if err != nil {
		return nil, err
	}
	conn, chans, reqs, err := ssh.NewClientConn(netConn, addr, config)
	if err != nil {
		return nil, err
	}
	nameCh := make(chan string)
	go func() {
		for r := range reqs {
			switch r.Type {
			case "@duplex-greeting":
				var greeting ssh_greetingPayload
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
	name := <-nameCh // todo: timeout nameCh
	go ssh_acceptChannels(chans, peer)
	return &ssh_peerConnection{
		endpoint: u.String(),
		name:     name,
		conn:     conn,
	}, nil
}

// channels

type ssh_channel struct {
	ssh.Channel
	ssh_channelData
}

func readFrame(r io.Reader) ([]byte, error) {
	bytes := make([]byte, 4)
	_, err := r.Read(bytes)
	if err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(bytes)
	frame := make([]byte, length)
	_, err = r.Read(frame)
	// TODO: handle errors based on written bytes
	if err != nil {
		return nil, err
	}
	return frame, nil
}

func writeFrame(w io.Writer, frame []byte) error {
	var buffer []byte
	n := uint32(len(frame))
	buffer = append(buffer, byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	buffer = append(buffer, frame...)
	_, err := w.Write(buffer)
	return err
}

func (c *ssh_channel) ReadFrame() ([]byte, error) {
	return readFrame(c)
}

func (c *ssh_channel) WriteFrame(frame []byte) error {
	return writeFrame(c, frame)
}

func (c *ssh_channel) ReadError() ([]byte, error) {
	return readFrame(c.Stderr())
}

func (c *ssh_channel) WriteError(frame []byte) error {
	return writeFrame(c.Stderr(), frame)
}

func (c *ssh_channel) Meta() ChannelMeta {
	return c
}

func (c *ssh_channel) Headers() []string {
	return c.ssh_channelData.Headers
}

func (c *ssh_channel) Service() string {
	return c.ssh_channelData.Service
}

func (c *ssh_channel) Join(rwc io.ReadWriteCloser) {
	go joinChannel(c, rwc)
}

func ssh_acceptChannels(chans <-chan ssh.NewChannel, peer *Peer) {
	var meta ssh_channelData
	for newCh := range chans {
		switch newCh.ChannelType() {
		case "@duplex":
			go func() {
				err := ssh.Unmarshal(newCh.ExtraData(), &meta)
				if err != nil {
					newCh.Reject(ssh.UnknownChannelType, "failed to parse channel data")
					return
				}
				if meta.Service == "" {
					newCh.Reject(ssh.UnknownChannelType, "empty service")
					return
				}
				ch, reqs, err := newCh.Accept()
				if err != nil {
					log.Println("debug: accept error:", err)
					return
				}
				go ssh.DiscardRequests(reqs)
				peer.incomingCh <- &ssh_channel{ch, meta}
			}()
		}
	}
}
