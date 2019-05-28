// +build linux

package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	unshareDescription = "Runs a command in a modified user namespace."
	_unshareCommand    = &cobra.Command{
		Use:   "unshare [flags] [COMMAND [ARG]]",
		Short: "Run a command in a modified user namespace",
		Long:  unshareDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			unshareCommand.InputArgs = args
			unshareCommand.GlobalFlags = MainGlobalOpts
			return unshareCmd(&unshareCommand)
		},
		Example: `podman unshare id
  podman unshare cat /proc/self/uid_map,
  podman unshare podman-script.sh`,
	}
	unshareCommand cliconfig.PodmanCommand
)

func init() {
	unshareCommand.Command = _unshareCommand
	unshareCommand.SetHelpTemplate(HelpTemplate())
	unshareCommand.SetUsageTemplate(UsageTemplate())
	flags := _unshareCommand.Flags()
	flags.SetInterspersed(false)
}

func unshareEnv(config *libpod.RuntimeConfig) []string {
	return append(os.Environ(), "_CONTAINERS_USERNS_CONFIGURED=done",
		fmt.Sprintf("CONTAINERS_GRAPHROOT=%s", config.StorageConfig.GraphRoot),
		fmt.Sprintf("CONTAINERS_RUNROOT=%s", config.StorageConfig.RunRoot))
}

// unshareCmd execs whatever using the ID mappings that we want to use for ourselves
func unshareCmd(c *cliconfig.PodmanCommand) error {

	if isRootless := rootless.IsRootless(); !isRootless {
		return errors.Errorf("please use unshare with rootless")
	}
	// exec the specified command, if there is one
	if len(c.InputArgs) < 1 {
		// try to exec the shell, if one's set
		shell, shellSet := os.LookupEnv("SHELL")
		if !shellSet {
			return errors.Errorf("no command specified and no $SHELL specified")
		}
		c.InputArgs = []string{shell}
	}

	runtime, err := libpodruntime.GetRuntime(getContext(), c)
	if err != nil {
		return err
	}
	runtimeConfig, err := runtime.GetConfig()
	if err != nil {
		return err
	}

	cmd := exec.Command(c.InputArgs[0], c.InputArgs[1:]...)
	cmd.Env = unshareEnv(runtimeConfig)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
