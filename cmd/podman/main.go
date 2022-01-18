package main

import (
	"fmt"
	"os"
	"strings"

	_ "github.com/containers/podman/v4/cmd/podman/completion"
	_ "github.com/containers/podman/v4/cmd/podman/containers"
	_ "github.com/containers/podman/v4/cmd/podman/generate"
	_ "github.com/containers/podman/v4/cmd/podman/healthcheck"
	_ "github.com/containers/podman/v4/cmd/podman/images"
	_ "github.com/containers/podman/v4/cmd/podman/machine"
	_ "github.com/containers/podman/v4/cmd/podman/manifest"
	_ "github.com/containers/podman/v4/cmd/podman/networks"
	_ "github.com/containers/podman/v4/cmd/podman/play"
	_ "github.com/containers/podman/v4/cmd/podman/pods"
	"github.com/containers/podman/v4/cmd/podman/registry"
	_ "github.com/containers/podman/v4/cmd/podman/secrets"
	_ "github.com/containers/podman/v4/cmd/podman/system"
	_ "github.com/containers/podman/v4/cmd/podman/system/connection"
	_ "github.com/containers/podman/v4/cmd/podman/volumes"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/terminal"
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

	rootCmd = parseCommands()

	Execute()
	os.Exit(0)
}

func parseCommands() *cobra.Command {
	cfg := registry.PodmanConfig()
	for _, c := range registry.Commands {
		if supported, found := c.Command.Annotations[registry.EngineMode]; found {
			if !strings.Contains(cfg.EngineMode.String(), supported) {
				continue
			}
		}

		// Command cannot be run rootless
		_, found := c.Command.Annotations[registry.UnshareNSRequired]
		if found {
			if rootless.IsRootless() && os.Getuid() != 0 && c.Command.Name() != "scp" {
				c.Command.RunE = func(cmd *cobra.Command, args []string) error {
					return fmt.Errorf("cannot run command %q in rootless mode, must execute `podman unshare` first", cmd.CommandPath())
				}
			}
		} else {
			_, found = c.Command.Annotations[registry.ParentNSRequired]
			if rootless.IsRootless() && found && c.Command.Name() != "scp" {
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
	if err := terminal.SetConsole(); err != nil {
		logrus.Error(err)
		os.Exit(1)
	}

	return rootCmd
}
