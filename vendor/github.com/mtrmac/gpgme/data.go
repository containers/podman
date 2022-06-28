package gpgme

// #include <string.h>
// #include <gpgme.h>
// #include <errno.h>
// #include "go_gpgme.h"
import "C"

import (
	"io"
	"os"
	"runtime"
	"unsafe"
)

const (
	SeekSet = C.SEEK_SET
	SeekCur = C.SEEK_CUR
	SeekEnd = C.SEEK_END
)

//export gogpgme_readfunc
func gogpgme_readfunc(handle, buffer unsafe.Pointer, size C.size_t) C.ssize_t {
	d := callbackLookup(uintptr(handle)).(*Data)
	if len(d.buf) < int(size) {
		d.buf = make([]byte, size)
	}
	n, err := d.r.Read(d.buf[:size])
	if err != nil && err != io.EOF {
		C.gpgme_err_set_errno(C.EIO)
		return -1
	}
	C.memcpy(buffer, unsafe.Pointer(&d.buf[0]), C.size_t(n))
	return C.ssize_t(n)
}

//export gogpgme_writefunc
func gogpgme_writefunc(handle, buffer unsafe.Pointer, size C.size_t) C.ssize_t {
	d := callbackLookup(uintptr(handle)).(*Data)
	if len(d.buf) < int(size) {
		d.buf = make([]byte, size)
	}
	C.memcpy(unsafe.Pointer(&d.buf[0]), buffer, C.size_t(size))
	n, err := d.w.Write(d.buf[:size])
	if err != nil && err != io.EOF {
		C.gpgme_err_set_errno(C.EIO)
		return -1
	}
	return C.ssize_t(n)
}

//export gogpgme_seekfunc
func gogpgme_seekfunc(handle unsafe.Pointer, offset C.gpgme_off_t, whence C.int) C.gpgme_off_t {
	d := callbackLookup(uintptr(handle)).(*Data)
	n, err := d.s.Seek(int64(offset), int(whence))
	if err != nil {
		C.gpgme_err_set_errno(C.EIO)
		return -1
	}
	return C.gpgme_off_t(n)
}

// The Data buffer used to communicate with GPGME
type Data struct {
	dh  C.gpgme_data_t // WARNING: Call runtime.KeepAlive(d) after ANY passing of d.dh to C
	buf []byte
	cbs C.struct_gpgme_data_cbs
	r   io.Reader
	w   io.Writer
	s   io.Seeker
	cbc uintptr // WARNING: Call runtime.KeepAlive(d) after ANY use of d.cbc in C (typically via d.dh)
}

func newData() *Data {
	d := &Data{}
	runtime.SetFinalizer(d, (*Data).Close)
	return d
}

// NewData returns a new memory based data buffer
func NewData() (*Data, error) {
	d := newData()
	return d, handleError(C.gpgme_data_new(&d.dh))
}

// NewDataFile returns a new file based data buffer
func NewDataFile(f *os.File) (*Data, error) {
	d := newData()
	return d, handleError(C.gpgme_data_new_from_fd(&d.dh, C.int(f.Fd())))
}

// NewDataBytes returns a new memory based data buffer that contains `b` bytes
func NewDataBytes(b []byte) (*Data, error) {
	d := newData()
	var cb *C.char
	if len(b) != 0 {
		cb = (*C.char)(unsafe.Pointer(&b[0]))
	}
	return d, handleError(C.gpgme_data_new_from_mem(&d.dh, cb, C.size_t(len(b)), 1))
}

// NewDataReader returns a new callback based data buffer
func NewDataReader(r io.Reader) (*Data, error) {
	d := newData()
	d.r = r
	d.cbs.read = C.gpgme_data_read_cb_t(C.gogpgme_readfunc)
	cbc := callbackAdd(d)
	d.cbc = cbc
	return d, handleError(C.gogpgme_data_new_from_cbs(&d.dh, &d.cbs, C.uintptr_t(cbc)))
}

// NewDataWriter returns a new callback based data buffer
func NewDataWriter(w io.Writer) (*Data, error) {
	d := newData()
	d.w = w
	d.cbs.write = C.gpgme_data_write_cb_t(C.gogpgme_writefunc)
	cbc := callbackAdd(d)
	d.cbc = cbc
	return d, handleError(C.gogpgme_data_new_from_cbs(&d.dh, &d.cbs, C.uintptr_t(cbc)))
}

// NewDataReadWriter returns a new callback based data buffer
func NewDataReadWriter(rw io.ReadWriter) (*Data, error) {
	d := newData()
	d.r = rw
	d.w = rw
	d.cbs.read = C.gpgme_data_read_cb_t(C.gogpgme_readfunc)
	d.cbs.write = C.gpgme_data_write_cb_t(C.gogpgme_writefunc)
	cbc := callbackAdd(d)
	d.cbc = cbc
	return d, handleError(C.gogpgme_data_new_from_cbs(&d.dh, &d.cbs, C.uintptr_t(cbc)))
}

// NewDataReadWriteSeeker returns a new callback based data buffer
func NewDataReadWriteSeeker(rw io.ReadWriteSeeker) (*Data, error) {
	d := newData()
	d.r = rw
	d.w = rw
	d.s = rw
	d.cbs.read = C.gpgme_data_read_cb_t(C.gogpgme_readfunc)
	d.cbs.write = C.gpgme_data_write_cb_t(C.gogpgme_writefunc)
	d.cbs.seek = C.gpgme_data_seek_cb_t(C.gogpgme_seekfunc)
	cbc := callbackAdd(d)
	d.cbc = cbc
	return d, handleError(C.gogpgme_data_new_from_cbs(&d.dh, &d.cbs, C.uintptr_t(cbc)))
}

// Close releases any resources associated with the data buffer
func (d *Data) Close() error {
	if d.dh == nil {
		return nil
	}
	if d.cbc > 0 {
		callbackDelete(d.cbc)
	}
	_, err := C.gpgme_data_release(d.dh)
	runtime.KeepAlive(d)
	d.dh = nil
	return err
}

func (d *Data) Write(p []byte) (int, error) {
	n, err := C.gpgme_data_write(d.dh, unsafe.Pointer(&p[0]), C.size_t(len(p)))
	runtime.KeepAlive(d)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, io.EOF
	}
	return int(n), nil
}

func (d *Data) Read(p []byte) (int, error) {
	n, err := C.gpgme_data_read(d.dh, unsafe.Pointer(&p[0]), C.size_t(len(p)))
	runtime.KeepAlive(d)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, io.EOF
	}
	return int(n), nil
}

func (d *Data) Seek(offset int64, whence int) (int64, error) {
	n, err := C.gogpgme_data_seek(d.dh, C.gpgme_off_t(offset), C.int(whence))
	runtime.KeepAlive(d)
	return int64(n), err
}

// Name returns the associated filename if any
func (d *Data) Name() string {
	res := C.GoString(C.gpgme_data_get_file_name(d.dh))
	runtime.KeepAlive(d)
	return res
}
