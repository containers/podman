package pods

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/cmd/podman/utils"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	systemdTimeout     uint
	systemdOptions     = entities.GenerateSystemdOptions{}
	systemdDescription = `Generate systemd units for a pod or container.
  The generated units can later be controlled via systemctl(1).`

	systemdCmd = &cobra.Command{
		Use:   "systemd [flags] CTR|POD",
		Short: "Generate systemd units.",
		Long:  systemdDescription,
		RunE:  systemd,
		Args:  cobra.MinimumNArgs(1),
		Example: `podman generate systemd CTR
  podman generate systemd --new --time 10 CTR
  podman generate systemd --files --name POD`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: systemdCmd,
		Parent:  generateCmd,
	})
	flags := systemdCmd.Flags()
	flags.BoolVarP(&systemdOptions.Name, "name", "n", false, "Use container/pod names instead of IDs")
	flags.BoolVarP(&systemdOptions.Files, "files", "f", false, "Generate .service files instead of printing to stdout")
	flags.UintVarP(&systemdTimeout, "time", "t", containerConfig.Engine.StopTimeout, "Stop timeout override")
	flags.StringVar(&systemdOptions.RestartPolicy, "restart-policy", "on-failure", "Systemd restart-policy")
	flags.BoolVarP(&systemdOptions.New, "new", "", false, "Create a new container instead of starting an existing one")
	flags.StringVar(&systemdOptions.ContainerPrefix, "container-prefix", "container", "Systemd unit name prefix for containers")
	flags.StringVar(&systemdOptions.PodPrefix, "pod-prefix", "pod", "Systemd unit name prefix for pods")
	flags.StringVar(&systemdOptions.Separator, "separator", "-", "Systemd unit name seperator between name/id and prefix")
	flags.SetNormalizeFunc(utils.AliasFlags)
}

func systemd(cmd *cobra.Command, args []string) error {
	if cmd.Flags().Changed("time") {
		systemdOptions.StopTimeout = &systemdTimeout
	}

	report, err := registry.ContainerEngine().GenerateSystemd(registry.GetContext(), args[0], systemdOptions)
	if err != nil {
		return err
	}

	fmt.Println(report.Output)
	return nil
}
