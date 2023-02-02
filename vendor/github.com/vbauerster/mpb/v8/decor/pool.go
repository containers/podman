package decor

import "sync"

var bytePool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, 16)
		return &b
	},
}
