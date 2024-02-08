package common

import (
	"github.com/containers/podman/v5/cmd/podman/registry"
)

var (
	// Pull in configured json library
	json = registry.JSONLibrary()
)
