package simplex

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
)

type MockConn struct {
	sync.Mutex

	sent           []*bytes.Buffer
	closed         bool
	inbox          chan string
	paired         *MockConn
	writes         sync.WaitGroup
	expectedWrites int
	reads          sync.WaitGroup
	expectedReads  int
}

func NewMockConn() *MockConn {
	return &MockConn{
		closed: false,
		inbox:  make(chan string, 1024),
	}
}

func (conn *MockConn) ExpectReads(n int) {
	conn.expectedReads = n
	conn.reads.Add(n)
}

func (conn *MockConn) ExpectWrites(n int) {
	conn.expectedWrites = n
	conn.writes.Add(n)
}

func (conn *MockConn) Close() error {
	conn.closed = true
	close(conn.inbox)
	return nil
}

func (conn *MockConn) Write(p []byte) (int, error) {
	conn.Lock()
	defer conn.Unlock()
	buf := bytes.NewBuffer(p)
	conn.sent = append(conn.sent, buf)
	if conn.paired != nil {
		conn.paired.inbox <- buf.String()
	}
	if os.Getenv("DEBUG") != "" {
		fmt.Println(buf.String())
	}
	if conn.expectedWrites > 0 {
		conn.writes.Done()
	}
	return buf.Len(), nil
}

func (conn *MockConn) Read(p []byte) (int, error) {
	if conn.expectedReads > 0 {
		defer conn.reads.Done()
	}
	v, ok := <-conn.inbox
	if !ok {
		return 0, fmt.Errorf("Inbox closed")
	}
	if os.Getenv("DEBUG") != "" {
		defer fmt.Println(v)
	}
	return copy(p, v), nil
}

func NewConnPair() (*MockConn, *MockConn) {
	conn1 := NewMockConn()
	conn2 := NewMockConn()
	conn1.paired = conn2
	conn2.paired = conn1
	return conn1, conn2
}

func NewPeerPair(rpc *RPC) (*Peer, *Peer) {
	conn1, conn2 := NewConnPair()
	var wg sync.WaitGroup
	var peer1, peer2 *Peer
	wg.Add(2)
	go func() {
		peer1, _ = rpc.Accept(conn1)
		wg.Done()
	}()
	go func() {
		peer2, _ = rpc.Handshake(conn2)
		wg.Done()
	}()
	wg.Wait()
	return peer1, peer2
}

func Echo(ch *Channel) error {
	var obj interface{}
	if _, err := ch.Recv(&obj); err != nil {
		return err
	}
	return ch.Send(obj, false)
}

func Generator(ch *Channel) error {
	var count float64
	if _, err := ch.Recv(&count); err != nil {
		return err
	}
	var i float64
	for i = 1; i <= count; i++ {
		m := map[string]float64{"num": i}
		if err := ch.Send(m, i != count); err != nil {
			return err
		}
	}
	return nil
}

func Adder(ch *Channel) error {
	var total float64 = 0
	var more bool = true
	var err error
	var count float64
	for more {
		if more, err = ch.Recv(&count); err != nil {
			return err
		}
		total += count
	}
	return ch.Send(total, false)
}

func Handshake(codec string) string {
	return fmt.Sprintf("%s/%s;%s", ProtocolName, ProtocolVersion, codec)
}

func Fatal(err error, t *testing.T) {
	if err != nil {
		t.Fatal(err)
	}
}

// Actual tests

func TestHandshake(t *testing.T) {
	conn := NewMockConn()
	defer conn.Close()
	rpc := NewRPC(NewJSONCodec())
	conn.inbox <- HandshakeAccept
	rpc.Handshake(conn)
	if conn.sent[0].String() != Handshake("json") {
		t.Fatal("Unexpected handshake frame:", conn.sent[0].String())
	}
}

func TestAccept(t *testing.T) {
	conn := NewMockConn()
	defer conn.Close()
	rpc := NewRPC(NewJSONCodec())
	conn.inbox <- Handshake("json")
	rpc.Accept(conn)
	if conn.sent[0].String() != HandshakeAccept {
		t.Fatal("Unexpected handshake response frame:", conn.sent[0].String())
	}
}

func TestRegisteredFuncAfterAccept(t *testing.T) {
	conn := NewMockConn()
	defer conn.Close()
	conn.ExpectWrites(2)
	rpc := NewRPC(NewJSONCodec())
	rpc.Register("echo", Echo)
	conn.inbox <- Handshake("json")
	rpc.Accept(conn)
	req := Message{
		Type:    TypeRequest,
		Method:  "echo",
		Id:      1,
		Payload: map[string]string{"foo": "bar"},
	}
	b, err := json.Marshal(req)
	Fatal(err, t)
	conn.inbox <- string(b)
	conn.writes.Wait()
	if len(conn.sent) != 2 {
		t.Fatal("Unexpected sent frame count:", len(conn.sent))
	}
}

func TestRegisteredFuncAfterHandshake(t *testing.T) {
	conn := NewMockConn()
	defer conn.Close()
	conn.ExpectWrites(2)
	rpc := NewRPC(NewJSONCodec())
	rpc.Register("echo", Echo)
	conn.inbox <- HandshakeAccept
	rpc.Handshake(conn)
	req := Message{
		Type:    TypeRequest,
		Method:  "echo",
		Id:      1,
		Payload: map[string]string{"foo": "bar"},
	}
	b, err := json.Marshal(req)
	Fatal(err, t)
	conn.inbox <- string(b)
	conn.writes.Wait()
	if len(conn.sent) != 2 {
		t.Fatal("Unexpected sent frame count:", len(conn.sent))
	}
}

func TestCallAfterHandshake(t *testing.T) {
	conn := NewMockConn()
	defer conn.Close()
	conn.ExpectWrites(2)
	rpc := NewRPC(NewJSONCodec())
	conn.inbox <- HandshakeAccept
	peer, err := rpc.Handshake(conn)
	Fatal(err, t)
	args := map[string]string{"foo": "bar"}
	expectedReply := map[string]string{"baz": "qux"}
	var reply map[string]interface{}
	b, err := json.Marshal(map[string]interface{}{
		"type":    TypeReply,
		"id":      1,
		"payload": expectedReply,
	})
	Fatal(err, t)
	go func() {
		conn.writes.Wait()
		conn.inbox <- string(b)
	}()
	err = peer.Call("foobar", args, &reply)
	Fatal(err, t)
	if reply["baz"] != expectedReply["baz"] {
		t.Fatal("Unexpted reply:", reply)
	}
}

func TestCallAfterAccept(t *testing.T) {
	conn := NewMockConn()
	defer conn.Close()
	conn.ExpectWrites(2)
	rpc := NewRPC(NewJSONCodec())
	conn.inbox <- Handshake("json")
	peer, err := rpc.Accept(conn)
	Fatal(err, t)
	args := map[string]string{"foo": "bar"}
	expectedReply := map[string]string{"baz": "qux"}
	var reply map[string]interface{}
	b, err := json.Marshal(map[string]interface{}{
		"type":    TypeReply,
		"id":      1,
		"payload": expectedReply,
	})
	Fatal(err, t)
	go func() {
		conn.writes.Wait()
		conn.inbox <- string(b)
	}()
	err = peer.Call("foobar", args, &reply)
	Fatal(err, t)
	if reply["baz"] != expectedReply["baz"] {
		t.Fatal("Unexpted reply:", reply)
	}
}

func TestAllOnPairedPeers(t *testing.T) {
	conn1, conn2 := NewConnPair()
	defer conn1.Close()
	defer conn2.Close()
	rpc := NewRPC(NewJSONCodec())
	rpc.Register("echo-tag", func(ch *Channel) error {
		var obj map[string]interface{}
		if _, err := ch.Recv(&obj); err != nil {
			return err
		}
		obj["tag"] = true
		return ch.Send(obj, false)
	})
	var wg sync.WaitGroup
	var peer1, peer2 *Peer
	var err error
	wg.Add(2)
	go func() {
		peer1, err = rpc.Accept(conn1)
		Fatal(err, t)
		wg.Done()
	}()
	go func() {
		peer2, err = rpc.Handshake(conn2)
		Fatal(err, t)
		wg.Done()
	}()
	wg.Wait()
	wg.Add(2)
	go func() {
		var reply map[string]interface{}
		err = peer1.Call("echo-tag", map[string]string{"from": "peer1"}, &reply)
		Fatal(err, t)
		if reply["from"] != "peer1" || reply["tag"] != true {
			t.Fatal("Unexpected reply to peer1:", reply)
		}
		wg.Done()
	}()
	go func() {
		var reply map[string]interface{}
		err = peer2.Call("echo-tag", map[string]string{"from": "peer2"}, &reply)
		Fatal(err, t)
		if reply["from"] != "peer2" || reply["tag"] != true {
			t.Fatal("Unexpected reply to peer2:", reply)
		}
		wg.Done()
	}()
	wg.Wait()
}

func TestStreamingMultipleResults(t *testing.T) {
	rpc := NewRPC(NewJSONCodec())
	rpc.Register("count", Generator)
	client, _ := NewPeerPair(rpc)
	ch := client.Open("count")
	err := ch.Send(5, false)
	Fatal(err, t)
	var reply map[string]interface{}
	var more bool = true
	var count float64 = 0
	loops := 0
	for more {
		more, err = ch.Recv(&reply)
		Fatal(err, t)
		count += reply["num"].(float64)
		loops += 1
		if loops > 5 {
			t.Fatal("Too many loops")
		}
	}
	if count != 15 {
		t.Fatal("Unexpected final count:", count)
	}
}

func TestStreamingMultipleArguments(t *testing.T) {
	rpc := NewRPC(NewJSONCodec())
	rpc.Register("adder", Adder)
	client, _ := NewPeerPair(rpc)
	ch := client.Open("adder")
	for i := 1; i <= 5; i++ {
		err := ch.Send(i, i != 5)
		Fatal(err, t)
	}
	var reply float64
	_, err := ch.Recv(&reply)
	Fatal(err, t)
	if reply != 15 {
		t.Fatal("Unexpected reply:", reply)
	}
}

func TestCustomCodec(t *testing.T) {
	b64json := &Codec{
		Name: "b64json",
		Encode: func(obj interface{}) ([]byte, error) {
			j, err := json.Marshal(obj)
			if err != nil {
				return nil, err
			}
			encoded := make([]byte, base64.StdEncoding.EncodedLen(len(j)))
			base64.StdEncoding.Encode(encoded, j)
			return encoded, nil
		},
		Decode: func(frame []byte, obj interface{}) error {
			decoded := make([]byte, base64.StdEncoding.DecodedLen(len(frame)))
			n, err := base64.StdEncoding.Decode(decoded, frame)
			if err != nil {
				return err
			}
			return json.Unmarshal(decoded[:n], obj)
		},
	}
	rpc := NewRPC(b64json)
	rpc.Register("echo", Echo)
	client, _ := NewPeerPair(rpc)
	var reply map[string]interface{}
	err := client.Call("echo", map[string]string{"foo": "bar"}, &reply)
	Fatal(err, t)
	if reply["foo"] != "bar" {
		t.Fatal("Unexpected reply:", reply)
	}
}

func TestHiddenExt(t *testing.T) {
	rpc := NewRPC(NewJSONCodec())
	rpc.Register("echo", Echo)
	client, server := NewPeerPair(rpc)
	ch := client.Open("echo")
	ch.SetExt(map[string]string{"hidden": "metadata"})
	err := ch.Send(map[string]string{"foo": "bar"}, false)
	Fatal(err, t)
	var reply map[string]interface{}
	_, err = ch.Recv(&reply)
	Fatal(err, t)
	if reply["foo"] != "bar" {
		t.Fatal("Unexpected reply:", reply)
	}
	var msg map[string]interface{}
	err = rpc.codec.Decode(server.conn.(*MockConn).sent[1].Bytes(), &msg)
	Fatal(err, t)
	if msg["ext"].(map[string]interface{})["hidden"] != "metadata" {
		t.Fatal("Unexpected ext:", msg["ext"])
	}
}
