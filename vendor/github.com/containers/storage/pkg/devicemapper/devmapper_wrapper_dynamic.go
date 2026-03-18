//go:build linux && cgo && !static_build
// +build linux,cgo,!static_build

package devicemapper

// #cgo pkg-config: devmapper
import "C"
