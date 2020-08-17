package registry

import (
	"os"
	"sync"

	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// Was --remote given on command line
	remoteOverride bool
	remoteSync     sync.Once
)

// IsRemote returns true if podman was built to run remote
// Use in init() functions as a initialization check
func IsRemote() bool {
	remoteSync.Do(func() {
		remote := &cobra.Command{}
		remote.Flags().BoolVarP(&remoteOverride, "remote", "r", false, "")
		_ = remote.ParseFlags(os.Args)
	})
	return podmanOptions.EngineMode == entities.TunnelMode || remoteOverride
}
