package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	restoreCommand     cliconfig.RestoreValues
	restoreDescription = `
   podman container restore

   Restores a container from a checkpoint. The container name or ID can be used.
`
	_restoreCommand = &cobra.Command{
		Use:   "restore [flags] CONTAINER [CONTAINER...]",
		Short: "Restores one or more containers from a checkpoint",
		Long:  restoreDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			restoreCommand.InputArgs = args
			restoreCommand.GlobalFlags = MainGlobalOpts
			restoreCommand.Remote = remoteclient
			return restoreCmd(&restoreCommand, cmd)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			return checkAllAndLatest(cmd, args, true)
		},
		Example: `podman container restore ctrID
  podman container restore --latest
  podman container restore --all`,
	}
)

func init() {
	restoreCommand.Command = _restoreCommand
	restoreCommand.SetHelpTemplate(HelpTemplate())
	restoreCommand.SetUsageTemplate(UsageTemplate())
	flags := restoreCommand.Flags()
	flags.BoolVarP(&restoreCommand.All, "all", "a", false, "Restore all checkpointed containers")
	flags.BoolVarP(&restoreCommand.Keep, "keep", "k", false, "Keep all temporary checkpoint files")
	flags.BoolVarP(&restoreCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.BoolVar(&restoreCommand.TcpEstablished, "tcp-established", false, "Restore a container with established TCP connections")
	flags.StringVarP(&restoreCommand.Import, "import", "i", "", "Restore from exported checkpoint archive (tar.gz)")
	flags.StringVarP(&restoreCommand.Name, "name", "n", "", "Specify new name for container restored from exported checkpoint (only works with --import)")
	flags.BoolVar(&restoreCommand.IgnoreRootfs, "ignore-rootfs", false, "Do not apply root file-system changes when importing from exported checkpoint")
	flags.BoolVar(&restoreCommand.IgnoreStaticIP, "ignore-static-ip", false, "Ignore IP address set via --static-ip")
	flags.BoolVar(&restoreCommand.IgnoreStaticMAC, "ignore-static-mac", false, "Ignore MAC address set via --mac-address")

	markFlagHiddenForRemoteClient("latest", flags)
}

func restoreCmd(c *cliconfig.RestoreValues, cmd *cobra.Command) error {
	if rootless.IsRootless() {
		return errors.New("restoring a container requires root")
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.DeferredShutdown(false)

	if c.Import == "" && c.IgnoreRootfs {
		return errors.Errorf("--ignore-rootfs can only be used with --import")
	}

	if c.Import == "" && c.Name != "" {
		return errors.Errorf("--name can only be used with --import")
	}

	if c.Name != "" && c.TcpEstablished {
		return errors.Errorf("--tcp-established cannot be used with --name")
	}

	argLen := len(c.InputArgs)
	if c.Import != "" {
		if c.All || c.Latest {
			return errors.Errorf("Cannot use --import with --all or --latest")
		}
		if argLen > 0 {
			return errors.Errorf("Cannot use --import with positional arguments")
		}
	}

	if (c.All || c.Latest) && argLen > 0 {
		return errors.Errorf("no arguments are needed with --all or --latest")
	}
	if argLen < 1 && !c.All && !c.Latest && c.Import == "" {
		return errors.Errorf("you must provide at least one name or id")
	}

	return runtime.Restore(getContext(), c)
}
