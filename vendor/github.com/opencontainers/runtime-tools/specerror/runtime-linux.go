package specerror

import (
	"fmt"

	rfc2119 "github.com/opencontainers/runtime-tools/error"
)

// define error codes
const (
	// DefaultRuntimeLinuxSymlinks represents "While creating the container (step 2 in the lifecycle), runtimes MUST create default symlinks if the source file exists after processing `mounts`."
	DefaultRuntimeLinuxSymlinks = "While creating the container (step 2 in the lifecycle), runtimes MUST create the default symlinks if the source file exists after processing `mounts`."
)

var (
	devSymbolicLinksRef = func(version string) (reference string, err error) {
		return fmt.Sprintf(referenceTemplate, version, "runtime-linux.md#dev-symbolic-links"), nil
	}
)

func init() {
	register(DefaultRuntimeLinuxSymlinks, rfc2119.Must, devSymbolicLinksRef)
}
