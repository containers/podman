//go:build !exclude_graphdriver_aufs && linux

package register

import (
	// register the aufs graphdriver
	_ "go.podman.io/storage/drivers/aufs"
)
