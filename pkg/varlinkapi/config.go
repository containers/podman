package varlinkapi

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	iopodman "github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod"
	"github.com/spf13/cobra"
)

// LibpodAPI is the basic varlink struct for libpod
type LibpodAPI struct {
	Cli *cobra.Command
	iopodman.VarlinkInterface
	Runtime *libpod.Runtime
}

// New creates a new varlink client
func New(cli *cliconfig.PodmanCommand, runtime *libpod.Runtime) *iopodman.VarlinkInterface {
	lp := LibpodAPI{Cli: cli.Command, Runtime: runtime}
	return iopodman.VarlinkNew(&lp)
}
