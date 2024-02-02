//go:build !remote

package libpod

import (
	"github.com/moby/sys/user"
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

func (c *Container) setProcessCapabilitiesExec(options *ExecOptions, user string, execUser *user.ExecUser, pspec *spec.Process) error {
	return nil
}
