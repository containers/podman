// +build linux,cgo,!gccgo

package unshare

// #cgo CFLAGS: -Wall
// extern void _buildah_unshare(void);
// void __attribute__((constructor)) init(void) {
//   _buildah_unshare();
// }
import "C"
