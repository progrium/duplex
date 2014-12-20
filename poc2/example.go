package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/progrium/duplex/poc2/duplex"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Not enough args")
		os.Exit(2)
	}
	uri := os.Args[1]

	peer1 := duplex.NewPeer()
	peer1.SetOption(duplex.OptName, "peer1")
	err := peer1.Bind(uri)
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
			println("peer1 recv:", string(frame))
			err = ch.WriteFrame(frame)
			if err != nil {
				panic(err)
			}
		}
	}()

	peer2 := duplex.NewPeer()
	peer2.SetOption(duplex.OptName, "peer2")
	err = peer2.Connect(uri)
	if err != nil {
		panic(err)
	}

	println("connected")
	fmt.Println(peer1.Peers(), peer2.Peers())
	target := peer2.Peers()[0]
	ch, err := peer2.Open(target, "foobar", []string{"abc=123"})
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			frame, err := ch.ReadFrame()
			if err != nil {
				panic(err)
			}
			println("peer2 recv:", string(frame))
		}
	}()

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		ch.WriteFrame(scanner.Bytes())
	}
}
