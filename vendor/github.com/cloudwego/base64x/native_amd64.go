/*
 * Copyright 2024 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
