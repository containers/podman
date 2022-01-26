package gpgme

import (
	"sync"
)

var callbacks struct {
	sync.Mutex
	m map[uintptr]interface{}
	c uintptr
}

func callbackAdd(v interface{}) uintptr {
	callbacks.Lock()
	defer callbacks.Unlock()
	if callbacks.m == nil {
		callbacks.m = make(map[uintptr]interface{})
	}
	callbacks.c++
	ret := callbacks.c
	callbacks.m[ret] = v
	return ret
}

func callbackLookup(c uintptr) interface{} {
	callbacks.Lock()
	defer callbacks.Unlock()
	ret := callbacks.m[c]
	if ret == nil {
		panic("callback pointer not found")
	}
	return ret
}

func callbackDelete(c uintptr) {
	callbacks.Lock()
	defer callbacks.Unlock()
	if callbacks.m[c] == nil {
		panic("callback pointer not found")
	}
	delete(callbacks.m, c)
}
