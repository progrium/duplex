package dpx

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func Reverse(in []byte) []byte {
	b := make([]byte, len(in))
	var j int = len(in) - 1
	for i := 0; i <= j; i++ {
		b[j-i] = in[i]
	}
	return b
}

func Call(peer *Peer, method string, arg interface{}) interface{} {
	ch := Open(peer, method)
	req := NewFrame(ch)
	req.Payload = arg.([]byte)
	req.Last = true
	SendFrame(ch, req)
	resp := ReceiveFrame(ch)
	return resp.Payload
}

func CallTo(peer *Peer, uuid string, method string, arg interface{}) interface{} {
	for {
		ch, err := OpenWith(peer, uuid, method)
		if err != nil {
			continue
		}
		req := NewFrame(ch)
		req.Payload = arg.([]byte)
		req.Last = true
		SendFrame(ch, req)
		resp := ReceiveFrame(ch)
		return resp.Payload
	}
}

type ExamplePayload struct {
	Message string
}

func TestPeerFrameSendReceive(t *testing.T) {
	s1 := NewPeer()
	if err := Bind(s1, "127.0.0.1:9876"); err != nil {
		t.Fatal(err)
	}
	s2 := NewPeer()
	if err := Connect(s2, "127.0.0.1:9876"); err != nil {
		t.Fatal(err)
	}

	clientChannel := Open(s1, "foobar")
	_, serverChannel := Accept(s2)

	if serverChannel.Method != clientChannel.Method {
		t.Fatal("channel method not matching", serverChannel.Method, clientChannel.Method)
	}

	clientInput := NewFrame(clientChannel)
	clientInput.Payload = []byte{1, 2, 3}
	clientInput.Last = true
	SendFrame(clientChannel, clientInput)
	serverInput := ReceiveFrame(serverChannel)

	if !bytes.Equal(serverInput.Payload, clientInput.Payload) {
		t.Fatal("input payloads not matching", serverInput.Payload, clientInput.Payload)
	}

	serverOutput := NewFrame(serverChannel)
	serverOutput.Payload = []byte{3, 2, 1}
	serverOutput.Last = true
	SendFrame(serverChannel, serverOutput)
	clientOutput := ReceiveFrame(clientChannel)

	if !bytes.Equal(serverOutput.Payload, clientOutput.Payload) {
		t.Fatal("output payloads not matching", serverOutput.Payload, clientOutput.Payload)
	}

	Close(s1)
	Close(s2)

}

func TestRPCishCallAccept(t *testing.T) {
	server := NewPeer()
	if err := Bind(server, "127.0.0.1:9877"); err != nil {
		t.Fatal(err)
	}
	client := NewPeer()
	if err := Connect(client, "127.0.0.1:9877"); err != nil {
		t.Fatal(err)
	}

	go func() {
		for {
			method, ch := Accept(server)
			if ch != nil && method == "foo" {
				req := ReceiveFrame(ch)
				resp := NewFrame(ch)
				resp.Payload = Reverse(req.Payload)
				SendFrame(ch, resp)
			}
		}
	}()

	resp := Call(client, "foo", []byte{1, 2, 3}).([]byte)
	if !bytes.Equal(resp, []byte{3, 2, 1}) {
		t.Fatal("response not reverse bytes")
	}

	Close(server)
	Close(client)
}

func TestRoundRobinAndAsyncConnect(t *testing.T) {
	client := NewPeer()
	if err := Connect(client, "127.0.0.1:9876"); err != nil {
		t.Fatal(err)
	}
	if err := Connect(client, "127.0.0.1:9875"); err != nil {
		t.Fatal(err)
	}
	if err := Bind(client, "127.0.0.1:9874"); err != nil {
		t.Fatal(err)
	}
	server1 := NewPeer()
	if err := Bind(server1, "127.0.0.1:9876"); err != nil {
		t.Fatal(err)
	}
	server2 := NewPeer()
	if err := Bind(server2, "127.0.0.1:9875"); err != nil {
		t.Fatal(err)
	}
	server3 := NewPeer()
	if err := Connect(server3, "127.0.0.1:9874"); err != nil {
		t.Fatal(err)
	}

	serverResponses := make(chan string, 4)
	serve := func(s *Peer, id string) {
		for {
			method, ch := Accept(s)
			if ch != nil && method == "foo" {
				serverResponses <- id
				req := ReceiveFrame(ch)
				resp := NewFrame(ch)
				resp.Payload = Reverse(req.Payload)
				SendFrame(ch, resp)
			}
		}
	}
	go serve(server1, "1")
	go serve(server2, "2")
	go serve(server3, "3")

	resp1 := Call(client, "foo", []byte{1, 2, 3}).([]byte)
	if !bytes.Equal(resp1, []byte{3, 2, 1}) {
		t.Fatal("response not reverse bytes")
	}
	resp2 := Call(client, "foo", []byte{1, 2, 3}).([]byte)
	if !bytes.Equal(resp2, []byte{3, 2, 1}) {
		t.Fatal("response not reverse bytes")
	}
	resp3 := Call(client, "foo", []byte{1, 2, 3}).([]byte)
	if !bytes.Equal(resp3, []byte{3, 2, 1}) {
		t.Fatal("response not reverse bytes")
	}
	resp4 := Call(client, "foo", []byte{1, 2, 3}).([]byte)
	if !bytes.Equal(resp4, []byte{3, 2, 1}) {
		t.Fatal("response not reverse bytes")
	}

	responses := ""
	for i := 0; i < 4; i++ {
		responses = responses + <-serverResponses
	}
	if !strings.Contains(responses, "1") {
		t.Fatal("server 1 did not get called")
	}
	if !strings.Contains(responses, "2") {
		t.Fatal("server 2 did not get called")
	}
	if !strings.Contains(responses, "3") {
		t.Fatal("server 3 did not get called")
	}

	Close(server1)
	Close(server2)
	Close(server3)
	Close(client)
}

func TestAsyncMessaging(t *testing.T) {
	client := NewPeer()
	if err := Connect(client, "127.0.0.1:9873"); err != nil {
		t.Fatal(err)
	}

	resp := make(chan []byte)
	go func() {
		resp <- Call(client, "foo", []byte{1, 2, 3}).([]byte)
	}()

	server := NewPeer()
	if err := Bind(server, "127.0.0.1:9873"); err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			method, ch := Accept(server)
			if ch != nil && method == "foo" {
				req := ReceiveFrame(ch)
				resp := NewFrame(ch)
				resp.Payload = Reverse(req.Payload)
				SendFrame(ch, resp)
			}
		}
	}()

	if !bytes.Equal(<-resp, []byte{3, 2, 1}) {
		t.Fatal("response not reverse bytes")
	}

	Close(server)
	Close(client)
}

func TestPeerSendReceiveCodec(t *testing.T) {
	s1 := NewPeer()
	if err := Bind(s1, "127.0.0.1:9876"); err != nil {
		t.Fatal(err)
	}
	s2 := NewPeer()
	if err := Connect(s2, "127.0.0.1:9876"); err != nil {
		t.Fatal(err)
	}

	clientChannel := Open(s1, "foobar")
	_, serverChannel := Accept(s2)

	input := new(ExamplePayload)
	input.Message = "Hello world"
	Send(clientChannel, input)
	output := &ExamplePayload{}
	err := Receive(serverChannel, output)
	if err != nil {
		t.Fatal(err.Error())
	}

	if input.Message != output.Message {
		t.Fatal("payloads not matching", input.Message, output.Message)
	}

	Close(s1)
	Close(s2)

}

func TestSpecificPeerConnect(t *testing.T) {
	client := NewPeer()
	if err := Connect(client, "127.0.0.1:9876"); err != nil {
		t.Fatal(err)
	}
	if err := Connect(client, "127.0.0.1:9875"); err != nil {
		t.Fatal(err)
	}
	if err := Bind(client, "127.0.0.1:9874"); err != nil {
		t.Fatal(err)
	}
	server1 := NewPeer()
	if err := Bind(server1, "127.0.0.1:9876"); err != nil {
		t.Fatal(err)
	}
	server2 := NewPeer()
	if err := Bind(server2, "127.0.0.1:9875"); err != nil {
		t.Fatal(err)
	}
	server3 := NewPeer()
	if err := Connect(server3, "127.0.0.1:9874"); err != nil {
		t.Fatal(err)
	}

	serverResponses := make(chan string, 1)

	serve := func(s *Peer) {
		for {
			method, ch := Accept(s)
			if ch != nil && method == "foo" {
				if ch.Target() != client.Name() {
					t.Fatal("client is not right", ch.Target(), client.Name())
				}
				serverResponses <- s.Name()
				req := ReceiveFrame(ch)
				resp := NewFrame(ch)
				resp.Payload = Reverse(req.Payload)
				SendFrame(ch, resp)
			}
		}
	}

	go serve(server1)
	go serve(server2)
	go serve(server3)

	resp1 := CallTo(client, server1.Name(), "foo", []byte{1, 2, 3}).([]byte)
	if !bytes.Equal(resp1, []byte{3, 2, 1}) {
		t.Fatal("response not reverse bytes")
	}

	if s := <-serverResponses; s != server1.Name() {
		t.Fatal("failed to call to specific server", s, server1.Name())
	}

	resp3 := CallTo(client, server3.Name(), "foo", []byte{1, 2, 3}).([]byte)
	if !bytes.Equal(resp3, []byte{3, 2, 1}) {
		t.Fatal("response not reverse bytes")
	}

	if s := <-serverResponses; s != server3.Name() {
		t.Fatal("failed to call to specific server", s, server3.Name())
	}

	resp2 := CallTo(client, server2.Name(), "foo", []byte{1, 2, 3}).([]byte)
	if !bytes.Equal(resp2, []byte{3, 2, 1}) {
		t.Fatal("response not reverse bytes")
	}

	if s := <-serverResponses; s != server2.Name() {
		t.Fatal("failed to call to specific server", s, server2.Name())
	}

	Close(server1)
	Close(server2)
	Close(server3)
	Close(client)
}

func TestPeerDropConnection(t *testing.T) {
	client := NewPeer()
	if err := Connect(client, "127.0.0.1:9876"); err != nil {
		t.Fatal(err)
	}
	if err := Connect(client, "127.0.0.1:9875"); err != nil {
		t.Fatal(err)
	}
	if err := Bind(client, "127.0.0.1:9874"); err != nil {
		t.Fatal(err)
	}
	server1 := NewPeer()
	if err := Bind(server1, "127.0.0.1:9876"); err != nil {
		t.Fatal(err)
	}
	server2 := NewPeer()
	if err := Bind(server2, "127.0.0.1:9875"); err != nil {
		t.Fatal(err)
	}
	server3 := NewPeer()
	if err := Connect(server3, "127.0.0.1:9874"); err != nil {
		t.Fatal(err)
	}

	serverResponses := make(chan string, 1)

	serve := func(s *Peer) {
		for {
			method, ch := Accept(s)
			if ch != nil && method == "foo" {
				if ch.Target() != client.Name() {
					t.Fatal("client is not right", ch.Target(), client.Name())
				}
				req := ReceiveFrame(ch)
				resp := NewFrame(ch)
				resp.Payload = Reverse(req.Payload)
				SendFrame(ch, resp)

				// drop the target
				time.Sleep(1 * time.Second)

				s.Drop(ch.Target())

				time.Sleep(1 * time.Second)
				serverResponses <- s.Name()
				break
			}
		}
	}

	go serve(server1)
	go serve(server2)
	go serve(server3)

	resp1 := CallTo(client, server1.Name(), "foo", []byte{1, 2, 3}).([]byte)
	if !bytes.Equal(resp1, []byte{3, 2, 1}) {
		t.Fatal("response not reverse bytes")
	}

	if s := <-serverResponses; s != server1.Name() {
		t.Fatal("failed to call to specific server", s, server1.Name())
	}

	if len(server1.Remote()) != 0 {
		t.Fatal("still servers in server1", server1.Remote())
	}

	for _, v := range client.Remote() {
		if v == server1.Name() {
			t.Fatal("server1 should have already been dropped")
		}
	}

	resp3 := CallTo(client, server3.Name(), "foo", []byte{1, 2, 3}).([]byte)
	if !bytes.Equal(resp3, []byte{3, 2, 1}) {
		t.Fatal("response not reverse bytes")
	}

	if s := <-serverResponses; s != server3.Name() {
		t.Fatal("failed to call to specific server", s, server3.Name())
	}

	for _, v := range client.Remote() {
		if v == server3.Name() {
			t.Fatal("server3 should have already been dropped")
		}
	}

	resp2 := CallTo(client, server2.Name(), "foo", []byte{1, 2, 3}).([]byte)
	if !bytes.Equal(resp2, []byte{3, 2, 1}) {
		t.Fatal("response not reverse bytes")
	}

	if s := <-serverResponses; s != server2.Name() {
		t.Fatal("failed to call to specific server", s, server2.Name())
	}

	for _, v := range client.Remote() {
		if v == server2.Name() {
			t.Fatal("server2 should have already been dropped")
		}
	}

	Close(server1)
	Close(server2)
	Close(server3)
	Close(client)
}
