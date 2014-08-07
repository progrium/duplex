package dpx

const (
	OpenFrame = 0
	DataFrame = 1
)

type Frame struct {
	_struct bool `codec:",toarray"`
	errCh   chan error
	chanRef *Channel

	Target string

	Type    int
	Channel int

	Method  string
	Headers map[string]string
	Error   string
	Last    bool

	Payload []byte
}

func newFrame(c *Channel) *Frame {
	return &Frame{
		chanRef: c,
		Channel: c.id,
		Headers: map[string]string{},
		errCh:   make(chan error),
	}
}
