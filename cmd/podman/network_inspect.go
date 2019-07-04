// +build !remoteclient

package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	networkinspectCommand     cliconfig.NetworkInspectValues
	networkinspectDescription = `Inspect network`
	_networkinspectCommand    = &cobra.Command{
		Use:   "inspect NETWORK [NETWORK...] [flags] ",
		Short: "network inspect",
		Long:  networkinspectDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			networkinspectCommand.InputArgs = args
			networkinspectCommand.GlobalFlags = MainGlobalOpts
			networkinspectCommand.Remote = remoteclient
			return networkinspectCmd(&networkinspectCommand)
		},
		Example: `podman network inspect podman`,
	}
)

func init() {
	networkinspectCommand.Command = _networkinspectCommand
	networkinspectCommand.SetHelpTemplate(HelpTemplate())
	networkinspectCommand.SetUsageTemplate(UsageTemplate())
}

func networkinspectCmd(c *cliconfig.NetworkInspectValues) error {
	if rootless.IsRootless() && !remoteclient {
		return errors.New("network inspect is not supported for rootless mode")
	}
	if len(c.InputArgs) < 1 {
		return errors.Errorf("at least one network name is required")
	}
	runtime, err := adapter.GetRuntimeNoStore(getContext(), &c.PodmanCommand)
	if err != nil {
		return err
	}
	return runtime.NetworkInspect(c)
}
