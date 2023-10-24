//go:build !remote
// +build !remote

package libpod

import (
	"github.com/containers/podman/v4/libpod/define"
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

func (c *Container) platformInspectContainerHostConfig(ctrSpec *spec.Spec, hostConfig *define.InspectContainerHostConfig) error {
	// Not sure what to put here. FreeBSD jails use pids from the
	// global pool but can only see their own pids.
	hostConfig.PidMode = "host"

	// UTS namespace mode
	hostConfig.UTSMode = c.NamespaceMode(spec.UTSNamespace, ctrSpec)

	return nil
}
