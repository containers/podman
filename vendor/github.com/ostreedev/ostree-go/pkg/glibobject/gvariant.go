/*
 * Copyright (c) 2013 Conformal Systems <info@conformal.com>
 *
 * This file originated from: http://opensource.conformal.com/
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package glibobject

// #cgo pkg-config: glib-2.0 gobject-2.0
// #include <glib.h>
// #include <glib-object.h>
// #include <gio/gio.h>
// #include "glibobject.go.h"
// #include <stdlib.h>
import "C"
import (
	"fmt"
	"unsafe"
)

/*
 * GVariant
 */

type GVariant struct {
	ptr unsafe.Pointer
}

//func GVariantNew(p unsafe.Pointer) *GVariant {
//o := &GVariant{p}
//runtime.SetFinalizer(o, (*GVariant).Unref)
//return o;
//}

//func GVariantNewSink(p unsafe.Pointer) *GVariant {
//o := &GVariant{p}
//runtime.SetFinalizer(o, (*GVariant).Unref)
//o.RefSink()
//return o;
//}

func (v *GVariant) native() *C.GVariant {
	return (*C.GVariant)(v.ptr)
}

func (v *GVariant) Ptr() unsafe.Pointer {
	return v.ptr
}

func (v *GVariant) Ref() {
	C.g_variant_ref(v.native())
}

func (v *GVariant) Unref() {
	C.g_variant_unref(v.native())
}

func (v *GVariant) RefSink() {
	C.g_variant_ref_sink(v.native())
}

func (v *GVariant) TypeString() string {
	cs := (*C.char)(C.g_variant_get_type_string(v.native()))
	return C.GoString(cs)
}

func (v *GVariant) GetChildValue(i int) *GVariant {
	cchild := C.g_variant_get_child_value(v.native(), C.gsize(i))
	return (*GVariant)(unsafe.Pointer(cchild))
}

func (v *GVariant) LookupString(key string) (string, error) {
	ckey := C.CString(key)
	defer C.free(unsafe.Pointer(ckey))
	// TODO: Find a way to have constant C strings in golang
	cstr := C._g_variant_lookup_string(v.native(), ckey)
	if cstr == nil {
		return "", fmt.Errorf("No such key: %s", key)
	}
	return C.GoString(cstr), nil
}

func ToGVariant(ptr unsafe.Pointer) *GVariant {
	return &GVariant{ptr}
}
