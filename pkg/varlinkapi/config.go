package varlinkapi

import (
	iopodman "github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod"
	"github.com/urfave/cli"
)

// LibpodAPI is the basic varlink struct for libpod
type LibpodAPI struct {
	Cli *cli.Context
	iopodman.VarlinkInterface
	Runtime *libpod.Runtime
}

// New creates a new varlink client
func New(cli *cli.Context, runtime *libpod.Runtime) *iopodman.VarlinkInterface {
	lp := LibpodAPI{Cli: cli, Runtime: runtime}
	return iopodman.VarlinkNew(&lp)
}
