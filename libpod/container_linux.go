// +build linux

package libpod

import (
	"github.com/containernetworking/plugins/pkg/ns"
)

type containerStateLinux struct {

	// NetNSPath is the path of the container's network namespace
	// Will only be set if config.CreateNetNS is true, or the container was
	// told to join another container's network namespace
	NetNS ns.NetNS `json:"-"`
}

// containerStateWindows is intentionally left as a blank stub
type containerStateWindows struct {
}

// containerStateDarwin is intentionally left as a blank stub
type containerStateDarwin struct {
}
