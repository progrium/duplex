package dpx

// #cgo LDFLAGS: -ldpx -lmsgpack
// #include <dpx.h>
// #include <stdlib.h>
import "C"

import (
	"net"
	"runtime"
	"strconv"
	"sync"
	"time"
	"unsafe"
)

var (
	init_mutex = &sync.Mutex{}
	has_init   = 0
)

type Peer struct {
	peer *C.dpx_peer
}

func newPeer() *Peer {
	init_mutex.Lock()
	defer init_mutex.Unlock()
	if has_init == 0 {
		C.dpx_init()
	}

	peer := C.dpx_peer_new()
	has_init++

	p := &Peer{
		peer: peer,
	}

	runtime.SetFinalizer(p, func(p *Peer) {
		C.dpx_peer_close(p.peer)
		go func() {
			time.Sleep(500 * time.Millisecond)
			C.dpx_peer_free(p.peer)

			init_mutex.Lock()
			defer init_mutex.Unlock()
			has_init--

			if has_init == 0 {
				C.dpx_cleanup()
			}
		}()

	})

	return p
}

func (p *Peer) Open(method string) *Channel {
	cMethod := C.CString(method)
	defer C.free(unsafe.Pointer(cMethod))
	cChan := C.dpx_peer_open(p.peer, cMethod)
	return fromCChannel(cChan)
}

func (p *Peer) Accept() *Channel {
	cChan := C.dpx_peer_accept(p.peer)
	return fromCChannel(cChan)
}

func (p *Peer) Close() error {
	return ParseError(int64(C.dpx_peer_close(p.peer)))
}

func (p *Peer) Connect(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}

	iport, err := strconv.Atoi(port)
	if err != nil {
		return err
	}

	chost := C.CString(host)
	defer C.free(unsafe.Pointer(chost))

	return ParseError(int64(C.dpx_peer_connect(p.peer, chost, C.int(iport))))
}

func (p *Peer) Bind(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}

	iport, err := strconv.Atoi(port)
	if err != nil {
		return err
	}

	chost := C.CString(host)
	defer C.free(unsafe.Pointer(chost))

	return ParseError(int64(C.dpx_peer_bind(p.peer, chost, C.int(iport))))
}
