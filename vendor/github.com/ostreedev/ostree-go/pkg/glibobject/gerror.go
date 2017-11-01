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
	"errors"
	"unsafe"
)

/*
 * GError
 */

// GError is a representation of GLib's GError
type GError struct {
	ptr unsafe.Pointer
}

func NewGError() GError {
	return GError{nil}
}

func (e GError) Ptr() unsafe.Pointer {
	if e.ptr == nil {
		return nil
	}
	return e.ptr
}

func (e GError) Nil() {
	e.ptr = nil
}

func (e *GError) native() *C.GError {
	if e == nil || e.ptr == nil {
		return nil
	}
	return (*C.GError)(e.ptr)
}

func ToGError(ptr unsafe.Pointer) GError {
	return GError{ptr}
}

func ConvertGError(e GError) error {
	defer C.g_error_free(e.native())
	return errors.New(C.GoString((*C.char)(C._g_error_get_message(e.native()))))
}
