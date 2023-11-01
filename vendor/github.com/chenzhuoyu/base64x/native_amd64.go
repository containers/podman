//go:generate make
package base64x

import (
    `unsafe`

    `github.com/bytedance/sonic/loader`
)

//go:nosplit
func b64encode(out *[]byte, src *[]byte, mode int) {
    __b64encode(noEscape(unsafe.Pointer(out)), noEscape(unsafe.Pointer(src)), mode)
}

//go:nosplit
func b64decode(out *[]byte, src unsafe.Pointer, len int, mode int) (ret int) {
    return __b64decode(noEscape(unsafe.Pointer(out)), noEscape(unsafe.Pointer(src)), len, mode)
}

// asm2asm templates
var (
    __b64encode func(out unsafe.Pointer, src unsafe.Pointer, mod int)
    __b64decode func(out unsafe.Pointer, src unsafe.Pointer, len int, mod int) (ret int)
)

// directly jump PCs
var (
    _subr__b64encode uintptr
    _subr__b64decode uintptr
)

var stubs = []loader.GoC{
    {"_b64encode", &_subr__b64encode, &__b64encode},
    {"_b64decode", &_subr__b64decode, &__b64decode},
}

func init() {
    if hasAVX2() {
        archFlags = _MODE_AVX2
    }
    loader.WrapGoC(text__native_entry__, funcs, stubs, "base64x", "base64x/native.c")
}
