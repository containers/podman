package common

import (
	"github.com/containers/podman/v6/cmd/podman/registry"
)

var (
	// Pull in configured json library
	json = registry.JSONLibrary()
)
