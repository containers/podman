//go:build plan9 || appengine || wasm
// +build plan9 appengine wasm

package flags

func getTerminalColumns() int {
	return 80
}
