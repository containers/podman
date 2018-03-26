package varlinkapi

import "github.com/projectatomic/libpod/cmd/podman/ioprojectatomicpodman"

// LibpodAPI is the basic varlink struct for libpod
type LibpodAPI struct {
	ioprojectatomicpodman.VarlinkInterface
}

var (
	lp = LibpodAPI{}
	// VarlinkLibpod instantiation
	VarlinkLibpod = ioprojectatomicpodman.VarlinkNew(&lp)
)
