package duplex

import (
	"bytes"
	"strings"
	"testing"
)

var transportBaseUris = []string{"inproc://test-", "unix:///tmp/test.", "tcp://127.0.0.1:666"}

func shutdownPeer(t *testing.T, peer *Peer) {
	err := peer.Shutdown()
	if err != nil {
		t.Fatal(err)
	}
}

func f(uri string) string {
	return strings.Split(uri, "//")[0]
}

func TestConnectPeers(t *testing.T) {
	// here we set up two peers and connect them together
	for _, uri := range transportBaseUris {
		server := NewPeer()
		client := NewPeer()
		defer shutdownPeer(t, server)
		defer shutdownPeer(t, client)
		// bind server peer to endpoint
		err := server.Bind(uri + "1")
		if err != nil {
			t.Fatal(f(uri), err)
		}
		// server SHOULD have no peers
		if len(server.Peers()) != 0 {
			t.Fatal(f(uri), "unexpected server peers:", server.Peers())
		}
		// connect client peer to above endpoint
		err = client.Connect(uri + "1")
		if err != nil {
			t.Fatal(f(uri), err)
		}
		// client SHOULD have server as peer
		if len(client.Peers()) != 1 || client.Peers()[0] != server.GetOption(OptName).(string) {
			t.Fatal(f(uri), "unexpected client peers:", client.Peers())
		}
		// server SHOULD have client as peer
		if len(server.Peers()) != 1 || server.Peers()[0] != client.GetOption(OptName).(string) {
			t.Fatal(f(uri), "unexpected server peers:", server.Peers())
		}
		// disconnect client from server
		err = client.Disconnect(uri + "1")
		if err != nil {
			t.Fatal(f(uri), err)
		}
		// server may not drop client immediately
		// so we drop the peer explicitly just in case
		server.Drop(client.GetOption(OptName).(string))
		// server and client SHOULD have no peers
		if len(client.Peers()) != 0 || len(server.Peers()) != 0 {
			t.Fatal(f(uri), "peers not disconnected:",
				"client:", client.Peers(),
				"server:", server.Peers())
		}
	}
}

func TestConnect3Peers(t *testing.T) {
	// here we set up three peers, two client peers connected to a "server" peer.
	// however the second client is connected to by the server.
	for _, uri := range transportBaseUris {
		server := NewPeer()
		client1 := NewPeer()
		client2 := NewPeer()
		defer shutdownPeer(t, server)
		defer shutdownPeer(t, client1)
		defer shutdownPeer(t, client2)
		// bind server to endpoint
		err := server.Bind(uri + "1")
		if err != nil {
			t.Fatal(f(uri), err)
		}
		// bind client2 to an endpoint
		err = client2.Bind(uri + "2")
		if err != nil {
			t.Fatal(f(uri), err)
		}
		// connect client1 to server
		err = client1.Connect(uri + "1")
		if err != nil {
			t.Fatal(f(uri), err)
		}
		// connect server to client2
		err = server.Connect(uri + "2")
		if err != nil {
			t.Fatal(f(uri), err)
		}
		// client1 and client2 SHOULD have server as peer
		if len(client1.Peers()) != 1 || client1.Peers()[0] != server.GetOption(OptName).(string) {
			t.Fatal(f(uri), "unexpected client1 peers:", client1.Peers())
		}
		if len(client2.Peers()) != 1 || client2.Peers()[0] != server.GetOption(OptName).(string) {
			t.Fatal(f(uri), "unexpected client2 peers:", client2.Peers())
		}
		// server SHOULD have both clients as peers
		if len(server.Peers()) != 2 ||
			!strings.Contains(strings.Join(server.Peers(), " "), client1.GetOption(OptName).(string)) ||
			!strings.Contains(strings.Join(server.Peers(), " "), client2.GetOption(OptName).(string)) {
			t.Fatal(f(uri), "unexpected server peers:", server.Peers())
		}
		// drop all peers from all peers
		server.Drop(client1.GetOption(OptName).(string))
		server.Drop(client2.GetOption(OptName).(string))
		client1.Drop(server.GetOption(OptName).(string))
		client2.Drop(server.GetOption(OptName).(string))
		// all peers SHOULD have no peers
		if len(client1.Peers()) != 0 || len(client2.Peers()) != 0 || len(server.Peers()) != 0 {
			t.Fatal(f(uri), "peers not disconnected:",
				"client1:", client1.Peers(),
				"client2:", client2.Peers(),
				"server:", server.Peers())
		}
	}
}

func TestOpenChannels(t *testing.T) {
	// two peers, we open a channel in both directions
	for _, uri := range transportBaseUris {
		server := NewPeer()
		client := NewPeer()
		defer shutdownPeer(t, server)
		defer shutdownPeer(t, client)
		// connect up the peers
		err := server.Bind(uri + "1")
		if err != nil {
			t.Fatal(f(uri), err)
		}
		err = client.Connect(uri + "1")
		if err != nil {
			t.Fatal(f(uri), err)
		}
		for i := 0; i < 2; i++ {
			// open a channel on one peer
			_, err := server.Open(client.GetOption(OptName).(string), "test-service", []string{"foo=bar"})
			if err != nil {
				t.Fatal(f(uri), err)
			}
			// accept it on the other peer
			meta, _ := client.Accept()
			if meta.Service() != "test-service" {
				t.Fatal(f(uri), "unexpected service on accepted channel:", meta.Service())
			}
			if len(meta.Headers()) != 1 {
				t.Fatal(f(uri), "unexpected headers on accepted channel:", meta.Headers())
			}
			// switch it up!
			client, server = server, client
		}
	}
}

func TestBalancedFrameChannels(t *testing.T) {
	// two servers, one client that connects to both,
	// channels are opened across both.
	// frames are echoed.
	for _, uri := range transportBaseUris {
		client := NewPeer()
		server1 := NewPeer()
		server2 := NewPeer()
		defer shutdownPeer(t, server1)
		defer shutdownPeer(t, server2)
		defer shutdownPeer(t, client)
		// connect up the peers
		err := server1.Bind(uri + "1")
		if err != nil {
			t.Fatal(f(uri), err)
		}
		err = server2.Bind(uri + "2")
		if err != nil {
			t.Fatal(f(uri), err)
		}
		err = client.Connect(uri + "1")
		if err != nil {
			t.Fatal(f(uri), err)
		}
		err = client.Connect(uri + "2")
		if err != nil {
			t.Fatal(f(uri), err)
		}
		// set up echo services
		echoOnceService := func(peer *Peer) {
			for {
				meta, ch := peer.Accept()
				if meta == nil {
					return
				}
				if meta.Service() != "echo" {
					t.Fatal(f(uri), "unexpected service on accepted channel:", meta.Service())
				}
				frame, err := ch.ReadFrame()
				if err != nil {
					t.Fatal(f(uri), err)
					return
				}
				err = ch.WriteFrame(frame)
				if err != nil {
					t.Fatal(f(uri), err)
					return
				}
			}
		}
		go echoOnceService(server1)
		go echoOnceService(server2)
		// balance some requests across the servers
		for i := 0; i < 4; i++ {
			ch, err := client.Open(client.NextPeer(), "echo", nil)
			if err != nil {
				t.Fatal(f(uri), err)
			}
			sentFrame := []byte("foobar")
			err = ch.WriteFrame(sentFrame)
			if err != nil {
				t.Fatal(f(uri), err)
			}
			rcvdFrame, err := ch.ReadFrame()
			if err != nil {
				t.Fatal(f(uri), err)
			}
			if !bytes.Equal(sentFrame, rcvdFrame) {
				t.Fatal(f(uri), "sent and received frames not matching:", sentFrame, rcvdFrame)
			}
		}
	}
}

func TestAttachedChannels(t *testing.T) {
	// two peers, we open a channel in both directions ...
	// then open a channel on that channel!
	for _, uri := range transportBaseUris {
		server := NewPeer()
		client := NewPeer()
		defer shutdownPeer(t, server)
		defer shutdownPeer(t, client)
		// connect up the peers
		err := server.Bind(uri + "1")
		if err != nil {
			t.Fatal(f(uri), err)
		}
		err = client.Connect(uri + "1")
		if err != nil {
			t.Fatal(f(uri), err)
		}
		for i := 0; i < 2; i++ {
			// open a channel on one peer
			opened, err := server.Open(client.GetOption(OptName).(string), "outer", nil)
			if err != nil {
				t.Fatal(f(uri), err)
			}
			// accept it on the other peer
			_, accepted := client.Accept()
			for ii := 0; ii < 2; ii++ {
				// open channel from accepted end to opened
				_, err := accepted.Open("inner", []string{"foo=bar", "bar=foo"})
				if err != nil {
					t.Fatal(f(uri), err)
				}
				// accept it on opened end
				meta, _ := opened.Accept()
				if meta.Service() != "inner" {
					t.Fatal(f(uri), "unexpected service on accepted channel:", meta.Service())
				}
				if len(meta.Headers()) != 2 {
					t.Fatal(f(uri), "unexpected headers on accepted channel:", meta.Headers())
				}
				// switch it up on inner level!
				accepted, opened = opened, accepted
			}
			// switch it up on outer level!
			client, server = server, client
		}
	}
}
