package registry

import (
	"os"
	"sync"

	"github.com/containers/libpod/v2/pkg/domain/entities"
	"github.com/spf13/pflag"
)

// Value for --remote given on command line
var remoteFromCLI = struct {
	Value bool
	sync  sync.Once
}{}

// IsRemote returns true if podman was built to run remote or --remote flag given on CLI
// Use in init() functions as a initialization check
func IsRemote() bool {
	remoteFromCLI.sync.Do(func() {
		fs := pflag.NewFlagSet("remote", pflag.ContinueOnError)
		fs.BoolVarP(&remoteFromCLI.Value, "remote", "r", false, "")
		fs.ParseErrorsWhitelist.UnknownFlags = true
		fs.SetInterspersed(false)
		_ = fs.Parse(os.Args[1:])
	})
	return podmanOptions.EngineMode == entities.TunnelMode || remoteFromCLI.Value
}
