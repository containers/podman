//go:build go1.24 && !go1.25 && goexperiment.swissmap
// +build go1.24,!go1.25,goexperiment.swissmap

package rt

import (
	"unsafe"
)

type GoMapIterator struct {
	K     unsafe.Pointer
	V     unsafe.Pointer
	T     *GoMapType
	It    unsafe.Pointer
}
