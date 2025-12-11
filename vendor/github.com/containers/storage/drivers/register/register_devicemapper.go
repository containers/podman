//go:build !exclude_graphdriver_devicemapper && linux && cgo
// +build !exclude_graphdriver_devicemapper,linux,cgo

package register

import (
	// register the devmapper graphdriver
	_ "github.com/containers/storage/drivers/devmapper"
)
