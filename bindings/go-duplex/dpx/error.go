package dpx

const (
	DpxErrNone  = 0
	DpxErrFatal = -50

	DpxErrFree = 1

	DpxErrChanClosed = 10
	DpxErrChanFrame  = 11

	DpxErrNetFatal  = 20
	DpxErrNetNotAll = 21

	DpxErrPeerClosed = 30

	DpxErrDuplexClosed = 40
)

func ParseError(err int64) error {
	if err == DpxErrNone {
		return nil
	}

	return &DpxError{err: err}
}

type DpxError struct {
	err int64
}

func (d *DpxError) Code() int64 {
	return d.err
}

func (d *DpxError) Error() string {
	switch d.err {
	case DpxErrFatal:
		return "dpx: extremely unspecific fatal error occurred"

	case DpxErrFree:
		return "dpx: object is already being free'd"

	case DpxErrChanClosed:
		return "dpx: channel is closed"
	case DpxErrChanFrame:
		return "dpx: failed to send frame"

	case DpxErrNetFatal:
		return "dpx: network failure"
	case DpxErrNetNotAll:
		return "dpx: not all bytes were sent"

	case DpxErrPeerClosed:
		return "dpx: peer closed"

	case DpxErrDuplexClosed:
		return "dpx: duplex connection closed"
	}
	return "dpx: something went horribly wrong"
}
