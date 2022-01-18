package common

import (
	"github.com/containers/podman/v4/cmd/podman/registry"
)

var (

	// DefaultImageVolume default value
	DefaultImageVolume = "bind"
	// Pull in configured json library
	json = registry.JSONLibrary()
)
