package duplex

import (
	"io"
)

func joinChannel(ch Channel, rwc io.ReadWriteCloser) {
	done := make(chan struct{})
	go func() {
		io.Copy(ch, rwc)
		ch.CloseWrite()
		close(done)
	}()
	io.Copy(rwc, ch)
	rwc.Close()
	<-done
}
