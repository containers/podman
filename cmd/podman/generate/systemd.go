package pods

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/utils"
	"github.com/containers/podman/v3/pkg/domain/entities"
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
		Use:               "systemd [options] {CONTAINER|POD}",
		Short:             "Generate systemd units.",
		Long:              systemdDescription,
		RunE:              systemd,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: common.AutocompleteContainersAndPods,
		Example: `podman generate systemd CTR
  podman generate systemd --new --time 10 CTR
  podman generate systemd --files --name POD`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: systemdCmd,
		Parent:  generateCmd,
	})
	flags := systemdCmd.Flags()
	flags.BoolVarP(&systemdOptions.Name, "name", "n", false, "Use container/pod names instead of IDs")
	flags.BoolVarP(&files, "files", "f", false, "Generate .service files instead of printing to stdout")

	timeFlagName := "time"
	flags.UintVarP(&systemdTimeout, timeFlagName, "t", containerConfig.Engine.StopTimeout, "Stop timeout override")
	_ = systemdCmd.RegisterFlagCompletionFunc(timeFlagName, completion.AutocompleteNone)
	flags.BoolVarP(&systemdOptions.New, "new", "", false, "Create a new container instead of starting an existing one")
	flags.BoolVarP(&systemdOptions.NoHeader, "no-header", "", false, "Skip header generation")

	containerPrefixFlagName := "container-prefix"
	flags.StringVar(&systemdOptions.ContainerPrefix, containerPrefixFlagName, "container", "Systemd unit name prefix for containers")
	_ = systemdCmd.RegisterFlagCompletionFunc(containerPrefixFlagName, completion.AutocompleteNone)

	podPrefixFlagName := "pod-prefix"
	flags.StringVar(&systemdOptions.PodPrefix, podPrefixFlagName, "pod", "Systemd unit name prefix for pods")
	_ = systemdCmd.RegisterFlagCompletionFunc(podPrefixFlagName, completion.AutocompleteNone)

	separatorFlagName := "separator"
	flags.StringVar(&systemdOptions.Separator, separatorFlagName, "-", "Systemd unit name separator between name/id and prefix")
	_ = systemdCmd.RegisterFlagCompletionFunc(separatorFlagName, completion.AutocompleteNone)

	restartPolicyFlagName := "restart-policy"
	flags.StringVar(&systemdOptions.RestartPolicy, restartPolicyFlagName, "on-failure", "Systemd restart-policy")
	_ = systemdCmd.RegisterFlagCompletionFunc(restartPolicyFlagName, common.AutocompleteSystemdRestartOptions)

	formatFlagName := "format"
	flags.StringVar(&format, formatFlagName, "", "Print the created units in specified format (json)")
	_ = systemdCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(nil))

	flags.SetNormalizeFunc(utils.TimeoutAliasFlags)
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
