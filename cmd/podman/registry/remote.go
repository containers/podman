package registry

import (
	"os"
	"sync"

	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/spf13/pflag"
)

// Value for --remote given on command line
var remoteFromCLI = struct {
	Value bool
	sync  sync.Once
}{}

// IsRemote returns true if podman was built to run remote or --remote flag given on CLI
// Use in init() functions as an initialization check
func IsRemote() bool {
	remoteFromCLI.sync.Do(func() {
		fs := pflag.NewFlagSet("remote", pflag.ContinueOnError)
		fs.ParseErrorsWhitelist.UnknownFlags = true
		fs.Usage = func() {}
		fs.SetInterspersed(false)
		fs.BoolVarP(&remoteFromCLI.Value, "remote", "r", false, "")
		_ = fs.Parse(os.Args[1:])
	})
	return podmanOptions.EngineMode == entities.TunnelMode || remoteFromCLI.Value
}
