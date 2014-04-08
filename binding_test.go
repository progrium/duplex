package duplex

import (
	"testing"
)

type Args struct {
	A, B int
}

type Reply struct {
	C int
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

	go func() {
		for {
			method, ch := server.Accept()
			if ch != nil && method == "Arith.Add" {
				args := new(Args)
				err := ch.Receive(args)
				if err != nil {
					t.Fatal(err.Error())
				}
				reply := new(Reply)
				reply.C = args.A + args.B
				ch.Send(reply)
			}
		}
	}()

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
