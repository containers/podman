//go:build !plan9 && !windows && (!js || !wasm)
// +build !plan9
// +build !windows
// +build !js !wasm

package sftp

import "syscall"

const S_IFMT = syscall.S_IFMT
