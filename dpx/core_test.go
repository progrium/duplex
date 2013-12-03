package dpx

import (
	"testing"
)

func TestQueue(t *testing.T) {
	q := NewQueue()
	q.Enqueue("foo")
	if q.Dequeue().(string) != "foo" {
		t.Fatal("dequeued value not foo")
	}
}

func TestSocket(t *testing.T) {
	s1 := Socket()
	if err := Bind(s1, "127.0.0.1:9876"); err != nil {
		t.Fatal(err)
	}
	s2 := Socket()
	if err := Connect(s2, "127.0.0.1:9876"); err != nil {
		t.Fatal(err)
	}

	openFrame := duplexframe{Type: OpenFrame, Channel: 1, Headers: map[string]string{}, Payload: []byte{}}
	inputFrame := duplexframe{Type: InputFrame, Channel: 1, Headers: map[string]string{}, Payload: []byte{1, 2, 3}}

	debugSend(s1, openFrame)
	debugSend(s1, inputFrame)
	got := debugReceive(s2)
	if got.Type != inputFrame.Type || got.Channel != inputFrame.Channel {
		t.Fatal("frame not received as sent")
	}

	debugSend(s2, openFrame)
	debugSend(s2, inputFrame)
	got = debugReceive(s1)
	if got.Type != inputFrame.Type || got.Channel != inputFrame.Channel {
		t.Fatal("frame not received as sent")
	}

	s3 := Socket()
	if err := Connect(s3, "127.0.0.1:9876"); err != nil {
		t.Fatal(err)
	}
	debugSend(s3, openFrame)
	debugSend(s3, inputFrame)
	got = debugReceive(s1)
	if got.Type != inputFrame.Type || got.Channel != inputFrame.Channel {
		t.Fatal("frame not received as sent")
	}

}
