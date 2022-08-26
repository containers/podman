//go:build linux
// +build linux

package libpod

import (
	"github.com/containernetworking/plugins/pkg/ns"
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

type containerPlatformState struct {
	// NetNSPath is the path of the container's network namespace
	// Will only be set if config.CreateNetNS is true, or the container was
	// told to join another container's network namespace
	NetNS ns.NetNS `json:"-"`
}

func networkDisabled(c *Container) (bool, error) {
	if c.config.CreateNetNS {
		return false, nil
	}
	if !c.config.PostConfigureNetNS {
		for _, ns := range c.config.Spec.Linux.Namespaces {
			if ns.Type == spec.NetworkNamespace {
				return ns.Path == "", nil
			}
		}
	}
	return false, nil
}
