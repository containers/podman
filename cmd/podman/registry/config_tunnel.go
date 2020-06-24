// +build remote

package registry

import (
	"os"
)

func init() {
	abiSupport = false

	// Enforce that podman-remote == podman --remote
	os.Args = append(os.Args, "--remote")
}
