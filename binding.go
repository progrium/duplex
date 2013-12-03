package duplex

import (
	"./dpx"
)

// This is the high level API in Go that mostly adds language specific
// conveniences around the core Duplex API that would eventually be in C

func NewSocket() {
	dpx.Socket()
}
