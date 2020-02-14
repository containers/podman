// +build !linux

// Signal handling for Linux only.
package signal

import (
	"fmt"
	"os"
	"syscall"
)

const SIGWINCH = syscall.Signal(0xff)

// ParseSignal translates a string to a valid syscall signal.
// It returns an error if the signal map doesn't include the given signal.
func ParseSignal(rawSignal string) (syscall.Signal, error) {
	return 0, fmt.Errorf("unsupported on non-linux platforms")
}

// CatchAll catches all signals and relays them to the specified channel.
func CatchAll(sigc chan os.Signal) {
	panic("Unsupported on non-linux platforms")
}

// StopCatch stops catching the signals and closes the specified channel.
func StopCatch(sigc chan os.Signal) {
	panic("Unsupported on non-linux platforms")
}
