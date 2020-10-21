package pods

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/utils"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	files              bool
	format             string
	systemdTimeout     uint
	systemdOptions     = entities.GenerateSystemdOptions{}
	systemdDescription = `Generate systemd units for a pod or container.
  The generated units can later be controlled via systemctl(1).`

	systemdCmd = &cobra.Command{
		Use:   "systemd [options] CTR|POD",
		Short: "Generate systemd units.",
		Long:  systemdDescription,
		RunE:  systemd,
		Args:  cobra.ExactArgs(1),
		Example: `podman generate systemd CTR
  podman generate systemd --new --time 10 CTR
  podman generate systemd --files --name POD`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: systemdCmd,
		Parent:  generateCmd,
	})
	flags := systemdCmd.Flags()
	flags.BoolVarP(&systemdOptions.Name, "name", "n", false, "Use container/pod names instead of IDs")
	flags.BoolVarP(&files, "files", "f", false, "Generate .service files instead of printing to stdout")
	flags.UintVarP(&systemdTimeout, "time", "t", containerConfig.Engine.StopTimeout, "Stop timeout override")
	flags.StringVar(&systemdOptions.RestartPolicy, "restart-policy", "on-failure", "Systemd restart-policy")
	flags.BoolVarP(&systemdOptions.New, "new", "", false, "Create a new container instead of starting an existing one")
	flags.StringVar(&systemdOptions.ContainerPrefix, "container-prefix", "container", "Systemd unit name prefix for containers")
	flags.StringVar(&systemdOptions.PodPrefix, "pod-prefix", "pod", "Systemd unit name prefix for pods")
	flags.StringVar(&systemdOptions.Separator, "separator", "-", "Systemd unit name separator between name/id and prefix")
	flags.StringVar(&format, "format", "", "Print the created units in specified format (json)")
	flags.SetNormalizeFunc(utils.AliasFlags)
}

func systemd(cmd *cobra.Command, args []string) error {
	if cmd.Flags().Changed("time") {
		systemdOptions.StopTimeout = &systemdTimeout
	}

	if registry.IsRemote() {
		logrus.Warnln("The generated units should be placed on your remote system")
	}

	reports, err := registry.ContainerEngine().GenerateSystemd(registry.GetContext(), args[0], systemdOptions)
	if err != nil {
		return err
	}

	if files {
		cwd, err := os.Getwd()
		if err != nil {
			return errors.Wrap(err, "error getting current working directory")
		}
		for name, content := range reports.Units {
			path := filepath.Join(cwd, fmt.Sprintf("%s.service", name))
			f, err := os.Create(path)
			if err != nil {
				return err
			}
			_, err = f.WriteString(content)
			if err != nil {
				return err
			}
			err = f.Close()
			if err != nil {
				return err
			}

			// add newline if default format is given
			if format == "" {
				path += "\n"
			}
			// modify in place so we can print the
			// paths when --files is set
			reports.Units[name] = path
		}
	}

	switch {
	case report.IsJSON(format):
		return printJSON(reports.Units)
	case format == "":
		return printDefault(reports.Units)
	default:
		return errors.Errorf("unknown --format argument: %s", format)
	}

}

func printDefault(units map[string]string) error {
	for _, content := range units {
		fmt.Print(content)
	}
	return nil
}

func printJSON(units map[string]string) error {
	b, err := json.MarshalIndent(units, "", " ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
