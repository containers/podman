// +build varlink

package varlinkapi

import (
	"github.com/containers/podman/v2/libpod"
	iopodman "github.com/containers/podman/v2/pkg/varlink"
	"github.com/spf13/cobra"
)

// VarlinkAPI is the basic varlink struct for libpod
type VarlinkAPI struct {
	Cli *cobra.Command
	iopodman.VarlinkInterface
	Runtime *libpod.Runtime
}

// New creates a new varlink client
func New(cli *cobra.Command, runtime *libpod.Runtime) *iopodman.VarlinkInterface {
	lp := VarlinkAPI{Cli: cli, Runtime: runtime}
	return iopodman.VarlinkNew(&lp)
}
