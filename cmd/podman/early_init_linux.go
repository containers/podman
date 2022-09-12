package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/containers/podman/v4/libpod/define"
)

func setRLimits() error {
	rlimits := new(syscall.Rlimit)
	rlimits.Cur = define.RLimitDefaultValue
	rlimits.Max = define.RLimitDefaultValue
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, rlimits); err != nil {
		if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, rlimits); err != nil {
			return fmt.Errorf("getting rlimits: %w", err)
		}
		rlimits.Cur = rlimits.Max
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, rlimits); err != nil {
			return fmt.Errorf("setting new rlimits: %w", err)
		}
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
