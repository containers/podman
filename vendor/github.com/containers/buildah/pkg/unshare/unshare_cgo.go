// +build linux,cgo,!gccgo

package unshare

// #cgo CFLAGS: -Wall
// extern void _containers_unshare(void);
// void __attribute__((constructor)) init(void) {
//   _containers_unshare();
// }
import "C"
