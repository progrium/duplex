package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/progrium/duplex/poc2/duplex"
)

func main() {
	peer1 := duplex.NewPeer()
	peer1.SetOption(duplex.OptPrivateKey, "~/.ssh/id_rsa")
	peer1.SetOption(duplex.OptName, "peer1")
	err := peer1.Bind("unix:///tmp/foo")
	if err != nil {
		panic(err)
	}

	go func() {
		meta, ch := peer1.Accept()
		fmt.Println("accepted", meta.Service(), meta.Headers())
		for {
			frame, err := ch.ReadFrame()
			if err != nil {
				panic(err)
			}
			println("server:", string(frame))
			err = ch.WriteFrame(frame)
			if err != nil {
				panic(err)
			}
		}
	}()

	peer2 := duplex.NewPeer()
	peer2.SetOption(duplex.OptPrivateKey, "~/.ssh/id_rsa")
	peer2.SetOption(duplex.OptName, "peer2")
	err = peer2.Connect("unix:///tmp/foo")
	if err != nil {
		panic(err)
	}

	println("connected")
	fmt.Println(peer1.Peers(), peer2.Peers())
	ch, err := peer2.Open(peer2.Peers()[0], "foobar", []string{"abc=123"})
	if err != nil {
		panic(err)
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		ch.WriteFrame(scanner.Bytes())
	}
}
