package base64x

import (
    `reflect`
    `unsafe`
)

func mem2str(v []byte) (s string) {
    (*reflect.StringHeader)(unsafe.Pointer(&s)).Len  = (*reflect.SliceHeader)(unsafe.Pointer(&v)).Len
    (*reflect.StringHeader)(unsafe.Pointer(&s)).Data = (*reflect.SliceHeader)(unsafe.Pointer(&v)).Data
    return
}

func str2mem(s string) (v []byte) {
    (*reflect.SliceHeader)(unsafe.Pointer(&v)).Cap  = (*reflect.StringHeader)(unsafe.Pointer(&s)).Len
    (*reflect.SliceHeader)(unsafe.Pointer(&v)).Len  = (*reflect.StringHeader)(unsafe.Pointer(&s)).Len
    (*reflect.SliceHeader)(unsafe.Pointer(&v)).Data = (*reflect.StringHeader)(unsafe.Pointer(&s)).Data
    return
}

func mem2addr(v []byte) unsafe.Pointer {
    return *(*unsafe.Pointer)(unsafe.Pointer(&v))
}

// NoEscape hides a pointer from escape analysis. NoEscape is
// the identity function but escape analysis doesn't think the
// output depends on the input. NoEscape is inlined and currently
// compiles down to zero instructions.
// USE CAREFULLY!
//go:nosplit
//goland:noinspection GoVetUnsafePointer
func noEscape(p unsafe.Pointer) unsafe.Pointer {
    x := uintptr(p)
    return unsafe.Pointer(x ^ 0)
}
