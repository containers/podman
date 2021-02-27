package common

import (
	"github.com/containers/podman/v3/cmd/podman/registry"
)

var (
	// DefaultHealthCheckInterval default value
	DefaultHealthCheckInterval = "30s"
	// DefaultHealthCheckRetries default value
	DefaultHealthCheckRetries uint = 3
	// DefaultHealthCheckStartPeriod default value
	DefaultHealthCheckStartPeriod = "0s"
	// DefaultHealthCheckTimeout default value
	DefaultHealthCheckTimeout = "30s"
	// DefaultImageVolume default value
	DefaultImageVolume = "bind"
	// Pull in configured json library
	json = registry.JSONLibrary()
)
