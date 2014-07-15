package duplex

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strconv"
	"strings"
	"testing"
	"time"
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

func makePair(t *testing.T) (*Peer, *Peer) {
	port := strconv.Itoa(rand.Intn(30) + 9860)

	client := NewPeer()
	if err := client.Bind("127.0.0.1:" + port); err != nil {
		t.Fatal(err)
	}
	server := NewPeer()
	if err := server.Connect("127.0.0.1:" + port); err != nil {
		t.Fatal(err)
	}
	return client, server
}

func TestSimpleCall(t *testing.T) {
	client, server := makePair(t)
	defer client.Close()
	defer server.Close()

	server.Register(new(Arith))
	go server.Serve()

	// Synchronous calls
	args := &Args{7, 8}
	reply := new(Reply)
	err := client.Call("Arith.Add", args, reply)
	if err != nil {
		t.Fatalf("Add: expected no error but got string %q", err.Error())
	}
	if reply.C != args.A+args.B {
		t.Fatalf("Add: expected %d got %d", reply.C, args.A+args.B)
	}
}

func TestAsyncCall(t *testing.T) {
	client, server := makePair(t)
	defer client.Close()
	defer server.Close()

	server.Register(new(Arith))
	go server.Serve()

	// Out of order.
	args := &Args{7, 8}
	mulReply := new(Reply)
	mulCall, _ := client.Open("Arith.Mul", args, mulReply)
	addReply := new(Reply)
	addCall, _ := client.Open("Arith.Add", args, addReply)

	addCall = <-addCall.Done
	if addCall.Error != nil {
		t.Fatalf("Add: expected no error but got string %q", addCall.Error.Error())
	}
	if addReply.C != args.A+args.B {
		t.Fatalf("Add: expected %d got %d", addReply.C, args.A+args.B)
	}

	mulCall = <-mulCall.Done
	if mulCall.Error != nil {
		t.Fatalf("Mul: expected no error but got string %q", mulCall.Error.Error())
	}
	if mulReply.C != args.A*args.B {
		t.Fatalf("Mul: expected %d got %d", mulReply.C, args.A*args.B)
	}
}

func TestCommonErrors(t *testing.T) {
	client, server := makePair(t)
	defer client.Close()
	defer server.Close()

	server.Register(new(Arith))
	go server.Serve()

	// Nonexistent method
	args := &Args{7, 0}
	reply := new(Reply)
	err := client.Call("Arith.BadOperation", args, reply)
	// expect an error
	if err == nil {
		t.Fatal("BadOperation: expected error")
	} else if strings.Index(err.Error(), "can't find") < 0 {
		t.Fatalf("BadOperation: expected can't find method error; got %q", err)
	}

	// Unknown service
	args = &Args{7, 8}
	reply = new(Reply)
	err = client.Call("Arith.Unknown", args, reply)
	if err == nil {
		t.Fatal("expected error calling unknown service")
	} else if strings.Index(err.Error(), "method") < 0 {
		t.Fatal("expected error about method; got", err)
	}

	// Error test
	args = &Args{7, 0}
	reply = new(Reply)
	err = client.Call("Arith.Div", args, reply)
	// expect an error: zero divide
	if err == nil {
		t.Fatal("Div: expected error")
	} else if strings.Index(err.Error(), "divide by zero") < 0 {
		t.Fatal("Div: expected divide by zero error; got", err)
	}

	// Non-struct argument
	const Val = 12345
	str := fmt.Sprint(Val)
	reply = new(Reply)
	err = client.Call("Arith.Scan", &str, reply)
	if err != nil {
		t.Fatalf("Scan: expected no error but got string %q", err.Error())
	} else if reply.C != Val {
		t.Fatalf("Scan: expected %d got %d", Val, reply.C)
	}

	// Non-struct reply
	args = &Args{27, 35}
	str = ""
	err = client.Call("Arith.String", args, &str)
	if err != nil {
		t.Fatalf("String: expected no error but got string %q", err.Error())
	}
	expect := fmt.Sprintf("%d+%d=%d", args.A, args.B, args.A+args.B)
	if str != expect {
		t.Fatalf("String: expected %s got %s", expect, str)
	}

	args = &Args{7, 8}
	reply = new(Reply)
	err = client.Call("Arith.Mul", args, reply)
	if err != nil {
		t.Fatalf("Mul: expected no error but got string %q", err.Error())
	}
	if reply.C != args.A*args.B {
		t.Fatalf("Mul: expected %d got %d", reply.C, args.A*args.B)
	}
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

func (t *StreamingArith) Sum(channel *Channel) error {
	args := new(StreamingArgs)
	sum := 0
	for {
		err := channel.Receive(args)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil
		}
		sum += args.A
	}
	channel.Send(&StreamingReply{C: sum})
	return nil
}

func (t *StreamingArith) Echo(channel *Channel) error {
	args := new(StreamingArgs)
	for {
		err := channel.Receive(args)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil
		}
		channel.Send(&StreamingReply{C: args.A, Index: args.Count})
	}
	return nil
}

func streamingCheck(t *testing.T, client *Peer) {
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
}

func TestStreamingOutput(t *testing.T) {
	client, server := makePair(t)
	defer client.Close()
	defer server.Close()

	server.Register(new(StreamingArith))
	go server.Serve()

	streamingCheck(t, client)

}

func TestStreamingInput(t *testing.T) {
	client, server := makePair(t)
	defer client.Close()
	defer server.Close()

	server.Register(new(StreamingArith))
	go server.Serve()

	input := new(SendStream)
	reply := new(StreamingReply)
	call, err := client.Open("StreamingArith.Sum", input, reply)
	if err != nil {
		t.Fatal(err.Error())
	}

	input.Send(&StreamingArgs{9, 0, 0})
	input.Send(&StreamingArgs{3, 0, 0})
	input.Send(&StreamingArgs{3, 0, 0})
	input.Send(&StreamingArgs{6, 0, 0})
	input.SendLast(&StreamingArgs{9, 0, 0})

	<-call.Done

	if call.Error != nil {
		t.Fatal("unexpected error:", call.Error.Error())
	}

	if reply.C != 30 {
		t.Fatal("Didn't receive the right sum value back:", reply.C)
	}

}

func TestStreamingInputOutput(t *testing.T) {
	client, server := makePair(t)
	defer client.Close()
	defer server.Close()

	server.Register(new(StreamingArith))
	go server.Serve()

	input := new(SendStream)
	output := make(chan *StreamingReply, 10)
	call, err := client.Open("StreamingArith.Echo", input, output)
	if err != nil {
		t.Fatal(err.Error())
	}

	count := 0
	two := make(chan int)
	go func() {
		for reply := range output {
			count += reply.Index
			if count == 2 {
				two <- count
			}
		}
	}()

	input.Send(&StreamingArgs{1, 1, 0})
	input.Send(&StreamingArgs{2, 1, 0})
	time.Sleep(100 * time.Millisecond)
	input.Send(&StreamingArgs{3, 1, 0})
	input.Send(&StreamingArgs{4, 1, 0})

	if (<-two) < 2 {
		t.Fatal("4 messages have been sent but only", count, "have been recieved")
	}
	input.SendLast(&StreamingArgs{5, 1, 0})

	<-call.Done

	if call.Error != nil {
		t.Fatal("unexpected error:", call.Error.Error())
	}

	if count != 5 {
		t.Fatal("Didn't receive the right number of values back:", count)
	}

}

func TestInterruptedCallByServer(t *testing.T) {
	client, server := makePair(t)
	defer client.Close()
	defer server.Close()

	server.Register(new(StreamingArith))
	go server.Serve()

	args := &StreamingArgs{3, 100, 30} // 30 elements back, then error
	output := make(chan *StreamingReply, 10)
	c, _ := client.Open("StreamingArith.Thrive", args, output)

	// check we get the error at the 30th call exactly
	count := 0
	for reply := range output {
		if reply.Index != count {
			t.Fatal("unexpected value:", reply.Index)
		}
		count += 1
	}
	if count != 30 {
		t.Fatal("received error before the right time:", count)
	}
	if strings.Index(c.Error.Error(), "Triggered error in middle") < 0 {
		t.Fatal("received wrong error message:", c.Error)
	}

	// make sure the wire is still in good shape
	streamingCheck(t, client)

	// then check a call that doesn't send anything, but errors out first
	args = &StreamingArgs{3, 100, 0}
	output = make(chan *StreamingReply, 10)
	c, _ = client.Open("StreamingArith.Thrive", args, output)
	_, ok := <-output
	if ok {
		t.Fatal("expected closed channel")
	}
	if strings.Index(c.Error.Error(), "Triggered error in middle") < 0 {
		t.Fatal("received wrong error message:", c.Error)
	}

	// make sure the wire is still in good shape
	streamingCheck(t, client)

}

func TestInterruptedCallByClient(t *testing.T) {
	client, server := makePair(t)
	defer client.Close()
	defer server.Close()

	server.Register(new(StreamingArith))
	go server.Serve()

	args := &StreamingArgs{3, 1000, -1}
	output := make(chan *StreamingReply, 10)
	c, _ := client.Open("StreamingArith.Thrive", args, output)
	go func() {
		count := 0
		for _ = range output {
			count++
		}
		if count == 1000 {
			t.Fatal("expected stream to be stopped by client")
		}
	}()
	err := c.Close()
	if err != nil {
		t.Fatal(err.Error())
	}
	<-c.Done

	// make sure the wire is still in good shape
	streamingCheck(t, client)
}
