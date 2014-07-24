package dpx

import (
	"bytes"
	"io"
	"strings"
	"testing"
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
	req := NewFrame()
	req.Payload = arg.([]byte)
	req.Last = true
	SendFrame(ch, req)
	resp := ReceiveFrame(ch)
	return resp.Payload
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

	if serverChannel.Method() != clientChannel.Method() {
		t.Fatal("channel method not matching", serverChannel.Method(), clientChannel.Method())
	}

	clientInput := NewFrame()
	clientInput.Payload = []byte{1, 2, 3}
	clientInput.Last = true
	SendFrame(clientChannel, clientInput)
	serverInput := ReceiveFrame(serverChannel)

	if !bytes.Equal(serverInput.Payload, clientInput.Payload) {
		t.Fatal("input payloads not matching", serverInput.Payload, clientInput.Payload)
	}

	serverOutput := NewFrame()
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
			if ch == nil {
				return
			}
			if ch != nil && method == "foo" {
				req := ReceiveFrame(ch)
				resp := NewFrame()
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
	if err := Connect(client, "127.0.0.1:9878"); err != nil {
		t.Fatal(err)
	}
	if err := Connect(client, "127.0.0.1:9875"); err != nil {
		t.Fatal(err)
	}
	if err := Bind(client, "127.0.0.1:9874"); err != nil {
		t.Fatal(err)
	}
	server1 := NewPeer()
	if err := Bind(server1, "127.0.0.1:9878"); err != nil {
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
			if ch == nil {
				return
			}
			if ch != nil && method == "foo" {
				serverResponses <- id
				req := ReceiveFrame(ch)
				resp := NewFrame()
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
			if ch == nil {
				return
			}
			if ch != nil && method == "foo" {
				req := ReceiveFrame(ch)
				resp := NewFrame()
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

type ExamplePayload struct {
	Message string
}

func TestPeerSendReceiveCodec(t *testing.T) {
	s1 := NewPeer()
	if err := Bind(s1, "127.0.0.1:9872"); err != nil {
		t.Fatal(err)
	}
	s2 := NewPeer()
	if err := Connect(s2, "127.0.0.1:9872"); err != nil {
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

type StreamingArgs struct {
	A       int
	Count   int
	ErrorAt int
}

type StreamingReply struct {
	C     int
	Index int
}

func TestPeerSendReceiveCodecAdvanced(t *testing.T) {
	s1 := NewPeer()
	if err := Bind(s1, "127.0.0.1:9871"); err != nil {
		t.Fatal(err)
	}
	s2 := NewPeer()
	if err := Connect(s2, "127.0.0.1:9871"); err != nil {
		t.Fatal(err)
	}

	defer s1.Close()
	defer s2.Close()

	client := s2.Open("Sum")
	server := s1.Accept()

	go func() {
		args := new(StreamingArgs)
		sum := 0
		for {
			err := Receive(server, args)
			if err == io.EOF {
				break
			} else if err != nil {
				SendErr(server, err.Error(), true)
				return
			}
			sum += args.A
		}
		SendLast(server, &StreamingReply{C: sum})
	}()

	Send(client, &StreamingArgs{9, 0, 0})
	Send(client, &StreamingArgs{3, 0, 0})
	Send(client, &StreamingArgs{3, 0, 0})
	Send(client, &StreamingArgs{6, 0, 0})
	SendLast(client, &StreamingArgs{9, 0, 0})

	reply := new(StreamingReply)
	err := Receive(client, reply)
	if err != nil {
		t.Fatal(err)
	}

	if err := client.Error(); err != nil {
		t.Fatal("failed to do tests:", err)
	}

	if reply.C != 30 {
		t.Fatal("Didn't receive the right sum value back:", reply.C)
	}
}
