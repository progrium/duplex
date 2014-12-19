package duplex

import (
	"errors"
)

func newInprocListener(addr string, peer *Peer) (Listener, error) {
	return nil, errors.New("not implemented")
}

func newInprocConnection(addr string, peer *Peer) (Connection, error) {
	return nil, errors.New("not implemented")
}
