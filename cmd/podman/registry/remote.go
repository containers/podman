package registry

import (
	"os"
	"sync"

	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/spf13/cobra"
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
		remote := false
		if _, ok := os.LookupEnv("CONTAINER_HOST"); ok {
			remote = true
		} else if _, ok := os.LookupEnv("CONTAINER_CONNECTION"); ok {
			remote = true
		}
		fs := pflag.NewFlagSet("remote", pflag.ContinueOnError)
		fs.ParseErrorsWhitelist.UnknownFlags = true
		fs.Usage = func() {}
		fs.SetInterspersed(false)
		fs.BoolVarP(&remoteFromCLI.Value, "remote", "r", remote, "")
		connectionFlagName := "connection"
		fs.StringP(connectionFlagName, "c", "", "")
		contextFlagName := "context"
		fs.String(contextFlagName, "", "")
		hostFlagName := "host"
		fs.StringP(hostFlagName, "H", "", "")
		urlFlagName := "url"
		fs.String(urlFlagName, "", "")

		// The shell completion logic will call a command called "__complete" or "__completeNoDesc"
		// This command will always be the second argument
		// To still parse --remote correctly in this case we have to set args offset to two in this case
		start := 1
		if len(os.Args) > 1 && (os.Args[1] == cobra.ShellCompRequestCmd || os.Args[1] == cobra.ShellCompNoDescRequestCmd) {
			start = 2
		}
		_ = fs.Parse(os.Args[start:])
		// --connection or --url implies --remote
		remoteFromCLI.Value = remoteFromCLI.Value || fs.Changed(connectionFlagName) || fs.Changed(urlFlagName) || fs.Changed(hostFlagName) || fs.Changed(contextFlagName)
	})
	return podmanOptions.EngineMode == entities.TunnelMode || remoteFromCLI.Value
}
