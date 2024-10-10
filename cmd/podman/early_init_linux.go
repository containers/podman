package main

import (
	"fmt"
	"os"
	"syscall"
)

func setRLimits() error {
	rlimits := new(syscall.Rlimit)
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, rlimits); err != nil {
		return fmt.Errorf("getting rlimits: %w", err)
	}
	rlimits.Cur = rlimits.Max
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, rlimits); err != nil {
		return fmt.Errorf("setting new rlimits: %w", err)
	}
	return nil
}

func setUMask() {
	// Be sure we can create directories with 0755 mode.
	syscall.Umask(0022)
}

func earlyInitHook() {
	if err := setRLimits(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set rlimits: %s\n", err.Error())
	}

	setUMask()
}
