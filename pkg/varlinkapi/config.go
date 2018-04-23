package varlinkapi

import (
	ioprojectatomicpodman "github.com/projectatomic/libpod/cmd/podman/varlink"
	"github.com/urfave/cli"
)

// LibpodAPI is the basic varlink struct for libpod
type LibpodAPI struct {
	Cli *cli.Context
	ioprojectatomicpodman.VarlinkInterface
}

// New creates a new varlink client
func New(cli *cli.Context) *ioprojectatomicpodman.VarlinkInterface {
	lp := LibpodAPI{Cli: cli}
	return ioprojectatomicpodman.VarlinkNew(&lp)
}
