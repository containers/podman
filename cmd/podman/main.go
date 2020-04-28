package main

import (
	"os"

	_ "github.com/containers/libpod/cmd/podman/containers"
	_ "github.com/containers/libpod/cmd/podman/healthcheck"
	_ "github.com/containers/libpod/cmd/podman/images"
	_ "github.com/containers/libpod/cmd/podman/manifest"
	_ "github.com/containers/libpod/cmd/podman/pods"
	"github.com/containers/libpod/cmd/podman/registry"
	_ "github.com/containers/libpod/cmd/podman/system"
	_ "github.com/containers/libpod/cmd/podman/volumes"
	"github.com/containers/storage/pkg/reexec"
)

func main() {
	if reexec.Init() {
		// We were invoked with a different argv[0] indicating that we
		// had a specific job to do as a subprocess, and it's done.
		return
	}

	cfg := registry.PodmanConfig()
	for _, c := range registry.Commands {
		for _, m := range c.Mode {
			if cfg.EngineMode == m {
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
