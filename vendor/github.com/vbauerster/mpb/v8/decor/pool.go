package decor

import "sync"

var bytesPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, 32)
		return &b
	},
}
