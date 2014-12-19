package duplex

import (
//"io"
)

type Channel interface {
	Write(data []byte) (int, error) // send
	Read(data []byte) (int, error)  // recv

	WriteFrame(data []byte) error // send_frame
	ReadFrame() ([]byte, error)   // recv_frame

	CloseWrite() error // send_end .. close_send?
	Close() error      // close

	//Open(chType, service string, headers []string) (Channel, error) // send_chan
	//Accept(chType string) (ChannelMeta, Channel)                    // recv_chan

	//Join(rwc io.ReadWriteCloser) // join(fd)
	//Errors() io.ReadWriter       // send_errframe + recv_errframe
}

type ChannelMeta interface {
	Service() string
	Headers() []string
}

type peerConnection interface {
	Disconnect() error
	Name() string
	Addr() string
	Open(service string, headers []string) (Channel, error)
}

type peerListener interface {
	Unbind() error
}
