package swag

import "unsafe"

type internalString struct {
	Data unsafe.Pointer
	Len  int
}

// hackStringBytes returns the (unsafe) underlying bytes slice of a string.
func hackStringBytes(str string) []byte {
	p := (*internalString)(unsafe.Pointer(&str)).Data
	return unsafe.Slice((*byte)(p), len(str))
}

/*
 * go1.20 version (for when go mod moves to a go1.20 requirement):

func hackStringBytes(str string) []byte {
	return unsafe.Slice(unsafe.StringData(str), len(str))
}
*/
