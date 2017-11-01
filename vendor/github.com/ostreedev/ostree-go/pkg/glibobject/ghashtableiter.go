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

import (
	"unsafe"
)

// #cgo pkg-config: glib-2.0 gobject-2.0
// #include <glib.h>
// #include <glib-object.h>
// #include <gio/gio.h>
// #include "glibobject.go.h"
// #include <stdlib.h>
import "C"

/*
 * GHashTableIter
 */
type GHashTableIter struct {
	ptr unsafe.Pointer
}

func (ht *GHashTableIter) Ptr() unsafe.Pointer {
	return ht.ptr
}

func (ht *GHashTableIter) native() *C.GHashTableIter {
	return (*C.GHashTableIter)(ht.ptr)
}

func ToGHashTableIter(ptr unsafe.Pointer) *GHashTableIter {
	return &GHashTableIter{ptr}
}
