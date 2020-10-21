package main

import (
	"fmt"
	"os"

	_ "github.com/containers/podman/v2/cmd/podman/containers"
	_ "github.com/containers/podman/v2/cmd/podman/generate"
	_ "github.com/containers/podman/v2/cmd/podman/healthcheck"
	_ "github.com/containers/podman/v2/cmd/podman/images"
	_ "github.com/containers/podman/v2/cmd/podman/manifest"
	_ "github.com/containers/podman/v2/cmd/podman/networks"
	_ "github.com/containers/podman/v2/cmd/podman/play"
	_ "github.com/containers/podman/v2/cmd/podman/pods"
	"github.com/containers/podman/v2/cmd/podman/registry"
	_ "github.com/containers/podman/v2/cmd/podman/system"
	_ "github.com/containers/podman/v2/cmd/podman/system/connection"
	_ "github.com/containers/podman/v2/cmd/podman/volumes"
	"github.com/containers/podman/v2/pkg/rootless"
	"github.com/containers/podman/v2/pkg/terminal"
	"github.com/containers/storage/pkg/reexec"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func main() {
	if reexec.Init() {
		// We were invoked with a different argv[0] indicating that we
		// had a specific job to do as a subprocess, and it's done.
		return
	}

	// Hard code TMPDIR functions to use /var/tmp, if user did not override
	if _, ok := os.LookupEnv("TMPDIR"); !ok {
		os.Setenv("TMPDIR", "/var/tmp")
	}

	cfg := registry.PodmanConfig()
	for _, c := range registry.Commands {
		for _, m := range c.Mode {
			if cfg.EngineMode == m {
				// Command cannot be run rootless
				_, found := c.Command.Annotations[registry.UnshareNSRequired]
				if found {
					if rootless.IsRootless() && found && os.Getuid() != 0 {
						c.Command.RunE = func(cmd *cobra.Command, args []string) error {
							return fmt.Errorf("cannot run command %q in rootless mode, must execute `podman unshare` first", cmd.CommandPath())
						}
					}
				} else {
					_, found = c.Command.Annotations[registry.ParentNSRequired]
					if rootless.IsRootless() && found {
						c.Command.RunE = func(cmd *cobra.Command, args []string) error {
							return fmt.Errorf("cannot run command %q in rootless mode", cmd.CommandPath())
						}
					}
				}
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
				c.Command.DisableFlagsInUseLine = true
			}
		}
	}
	if err := terminal.SetConsole(); err != nil {
		logrus.Error(err)
		os.Exit(1)
	}

	Execute()
	os.Exit(0)
}
