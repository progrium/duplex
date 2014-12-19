package duplex

import (
	"io"
)

type Channel interface {
	Write(data []byte) (int, error) // send
	Read(data []byte) (int, error)  // recv

	CloseWrite() error // send_end .. close_send?
	Close() error      // close

	Open(chType, service string, headers []string) (Channel, error) // send_chan
	Accept(chType string) (ChannelMeta, Channel)                    // recv_chan

	Join(rwc io.ReadWriteCloser) // join(fd)
	Errors() io.ReadWriter       // send_err + recv_err
}

type ChannelMeta interface {
	Service() string
	Headers() []string
	Type() string
}

type Connection interface {
	Disconnect() error
	Name() string
	Addr() string
	Open(chType, service string, headers []string) (Channel, error)
}

type Listener interface {
	Unbind() error
}
