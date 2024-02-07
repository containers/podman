//go:build !linux && !freebsd

package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/containers/podman/v5/cmd/podman/registry"
)

func syslogHook() {
	if !registry.PodmanConfig().Syslog {
		return
	}

	fmt.Fprintf(os.Stderr, "Logging to Syslog is not supported on %s", runtime.GOOS)
	os.Exit(1)
}
