package dpx

const (
	OpenFrame = 0
	DataFrame = 1
)

type Frame struct {
	_struct bool `codec:",toarray"`
	errCh   chan error
	chanRef *Channel

	Type    int
	Channel int

	Method  string
	Headers map[string]string

	Payload []byte
	Error   string
	Last    bool
}
