// +build remoteclient

package adapter

import "github.com/urfave/cli"

// RemoteRuntime describes a wrapper runtime struct
type RemoteRuntime struct{}

// LocalRuntime describes a typical libpod runtime
type LocalRuntime struct {
	Runtime *RemoteRuntime
	Remote  bool
}

// GetRuntime returns a LocalRuntime struct with the actual runtime embedded in it
func GetRuntime(c *cli.Context) (*LocalRuntime, error) {
	runtime := RemoteRuntime{}
	return &LocalRuntime{
		Runtime: &runtime,
		Remote:  true,
	}, nil
}

// Shutdown is a bogus wrapper for compat with the libpod runtime
func (r RemoteRuntime) Shutdown(force bool) error {
	return nil
}
