package duplex

import (
	"errors"
	"fmt"
	"testing"
)

type Args struct {
	A, B int
}

type Reply struct {
	C int
}

type Arith int

// Some of Arith's methods have value args, some have pointer args. That's deliberate.

func (t *Arith) Add(args Args, reply *Reply) error {
	reply.C = args.A + args.B
	return nil
}

func (t *Arith) Mul(args *Args, reply *Reply) error {
	reply.C = args.A * args.B
	return nil
}

func (t *Arith) Div(args Args, reply *Reply) error {
	if args.B == 0 {
		return errors.New("divide by zero")
	}
	reply.C = args.A / args.B
	return nil
}

func (t *Arith) String(args *Args, reply *string) error {
	*reply = fmt.Sprintf("%d+%d=%d", args.A, args.B, args.A+args.B)
	return nil
}

func (t *Arith) Scan(args string, reply *Reply) (err error) {
	_, err = fmt.Sscan(args, &reply.C)
	return
}

func (t *Arith) Error(args *Args, reply *Reply) error {
	panic("ERROR")
}

func (t *Arith) TakesContext(context *string, args string, reply *string) error {
	return nil
}

func TestSimpleCall(t *testing.T) {
	client := NewPeer()
	if err := client.Bind("127.0.0.1:9876"); err != nil {
		t.Fatal(err)
	}
	server := NewPeer()
	if err := server.Connect("127.0.0.1:9876"); err != nil {
		t.Fatal(err)
	}

	server.Register(new(Arith))
	go server.Serve()

	// Synchronous calls
	args := &Args{7, 8}
	reply := new(Reply)
	err := client.Call("Arith.Add", args, reply)
	if err != nil {
		t.Errorf("Add: expected no error but got string %q", err.Error())
	}
	if reply.C != args.A+args.B {
		t.Errorf("Add: expected %d got %d", reply.C, args.A+args.B)
	}

	client.Close()
	server.Close()
}

type StreamingArgs struct {
	A     int
	Count int
	// next two values have to be between 0 and Count-2 to trigger anything
	ErrorAt int // will trigger an error at the given spot,
}

type StreamingReply struct {
	C     int
	Index int
}

type StreamingArith int

func (t *StreamingArith) Thrive(args StreamingArgs, stream SendStream) error {
	for i := 0; i < args.Count; i++ {
		if i == args.ErrorAt {
			return errors.New("Triggered error in middle")
		}
		err := stream.Send(&StreamingReply{C: args.A, Index: i})
		if err != nil {
			return nil
		}
	}

	return nil
}

func TestStreamingOutput(t *testing.T) {
	client := NewPeer()
	if err := client.Bind("127.0.0.1:9876"); err != nil {
		t.Fatal(err)
	}
	server := NewPeer()
	if err := server.Connect("127.0.0.1:9876"); err != nil {
		t.Fatal(err)
	}

	server.Register(new(StreamingArith))
	go server.Serve()

	args := &StreamingArgs{3, 5, -1}
	replyChan := make(chan *StreamingReply, 10)
	call, _ := client.Open("StreamingArith.Thrive", args, replyChan)

	count := 0
	for reply := range replyChan {
		if reply.Index != count {
			t.Fatal("unexpected value:", reply.Index)
		}
		count += 1
	}

	if call.Error != nil {
		t.Fatal("unexpected error:", call.Error.Error())
	}

	if count != 5 {
		t.Fatal("Didn't receive the right number of packets back:", count)
	}

	client.Close()
	server.Close()
}
