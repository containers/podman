// +build linux

package main

import (
	"os"
	"os/exec"

	"github.com/containers/buildah/pkg/unshare"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	unshareDescription = "Runs a command in a modified user namespace."
	_unshareCommand    = &cobra.Command{
		Use:   "unshare [flags] [COMMAND [ARG]]",
		Short: "Run a command in a modified user namespace",
		Long:  unshareDescription,
		RunE:  unshareCmd,
		Example: `podman unshare id
  podman unshare cat /proc/self/uid_map,
  podman unshare podman-script.sh`,
	}
)

func init() {
	_unshareCommand.SetUsageTemplate(UsageTemplate())
	flags := _unshareCommand.Flags()
	flags.SetInterspersed(false)
}

// unshareCmd execs whatever using the ID mappings that we want to use for ourselves
func unshareCmd(c *cobra.Command, args []string) error {
	if isRootless := unshare.IsRootless(); !isRootless {
		return errors.Errorf("please use unshare with rootless")
	}
	// exec the specified command, if there is one
	if len(args) < 1 {
		// try to exec the shell, if one's set
		shell, shellSet := os.LookupEnv("SHELL")
		if !shellSet {
			return errors.Errorf("no command specified and no $SHELL specified")
		}
		args = []string{shell}
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = unshare.RootlessEnv()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	unshare.ExecRunnable(cmd)
	return nil
}
