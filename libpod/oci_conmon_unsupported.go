//go:build !linux && !freebsd
// +build !linux,!freebsd

package libpod

import (
	"errors"

	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/resize"
)

// Make a new Conmon-based OCI runtime with the given options.
// Conmon will wrap the given OCI runtime, which can be `runc`, `crun`, or
// any runtime with a runc-compatible CLI.
// The first path that points to a valid executable will be used.
// Deliberately private. Someone should not be able to construct this outside of
// libpod.
func newConmonOCIRuntime(name string, paths []string, conmonPath string, runtimeFlags []string, runtimeCfg *config.Config) (OCIRuntime, error) {
	return nil, errors.New("newConmonOCIRuntime not supported on this platform")
}

func registerResizeFunc(r <-chan resize.TerminalSize, bundlePath string) {
}
