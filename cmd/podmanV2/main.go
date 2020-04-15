package main

import (
	"os"

	_ "github.com/containers/libpod/cmd/podmanV2/containers"
	_ "github.com/containers/libpod/cmd/podmanV2/healthcheck"
	_ "github.com/containers/libpod/cmd/podmanV2/images"
	_ "github.com/containers/libpod/cmd/podmanV2/networks"
	_ "github.com/containers/libpod/cmd/podmanV2/pods"
	"github.com/containers/libpod/cmd/podmanV2/registry"
	_ "github.com/containers/libpod/cmd/podmanV2/system"
	_ "github.com/containers/libpod/cmd/podmanV2/volumes"
	"github.com/containers/storage/pkg/reexec"
)

func init() {
	// This is the bootstrap configuration, if user gives
	// CLI flags parts of this configuration may be overwritten
	registry.PodmanOptions = registry.NewPodmanConfig()
}

func main() {
	if reexec.Init() {
		// We were invoked with a different argv[0] indicating that we
		// had a specific job to do as a subprocess, and it's done.
		return
	}

	for _, c := range registry.Commands {
		for _, m := range c.Mode {
			if registry.PodmanOptions.EngineMode == m {
				parent := rootCmd
				if c.Parent != nil {
					parent = c.Parent
				}
				parent.AddCommand(c.Command)

				// - templates need to be set here, as PersistentPreRunE() is
				// not called when --help is used.
				// - rootCmd uses cobra default template not ours
				c.Command.SetHelpTemplate(helpTemplate)
				c.Command.SetUsageTemplate(usageTemplate)
			}
		}
	}

	Execute()
	os.Exit(0)
}
