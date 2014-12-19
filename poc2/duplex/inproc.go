package duplex

import (
	"errors"
)

func newInprocListener(peer *Peer, addr string) (Listener, error) {
	return nil, errors.New("not implemented")
}

func newInprocConnection(peer *Peer, addr string) (Connection, error) {
	return nil, errors.New("not implemented")
}
