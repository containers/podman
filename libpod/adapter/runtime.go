// +build !remoteclient

package adapter

import (
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/urfave/cli"
)

// LocalRuntime describes a typical libpod runtime
type LocalRuntime struct {
	Runtime *libpod.Runtime
	Remote  bool
}

// GetRuntime returns a LocalRuntime struct with the actual runtime embedded in it
func GetRuntime(c *cli.Context) (*LocalRuntime, error) {
	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return nil, err
	}
	return &LocalRuntime{
		Runtime: runtime,
	}, nil
}
