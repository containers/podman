package system

import (
	"fmt"
	"os"
	"text/template"

	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/validate"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	infoDescription = `Display information pertaining to the host, current storage stats, and build of podman.

  Useful for the user and when reporting issues.
`
	infoCommand = &cobra.Command{
		Use:     "info [options]",
		Args:    validate.NoArgs,
		Long:    infoDescription,
		Short:   "Display podman system information",
		RunE:    info,
		Example: `podman info`,
	}

	systemInfoCommand = &cobra.Command{
		Args:    infoCommand.Args,
		Use:     infoCommand.Use,
		Short:   infoCommand.Short,
		Long:    infoCommand.Long,
		RunE:    infoCommand.RunE,
		Example: `podman system info`,
	}
)

var (
	inFormat string
	debug    bool
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: infoCommand,
	})
	infoFlags(infoCommand.Flags())

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: systemInfoCommand,
		Parent:  systemCmd,
	})
	infoFlags(systemInfoCommand.Flags())
}

func infoFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&debug, "debug", "D", false, "Display additional debug information")
	flags.StringVarP(&inFormat, "format", "f", "", "Change the output format to JSON or a Go template")
}

func info(cmd *cobra.Command, args []string) error {
	info, err := registry.ContainerEngine().Info(registry.GetContext())
	if err != nil {
		return err
	}

	switch {
	case report.IsJSON(inFormat):
		b, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	case cmd.Flags().Changed("format"):
		tmpl, err := template.New("info").Parse(inFormat)
		if err != nil {
			return err
		}
		return tmpl.Execute(os.Stdout, info)
	default:
		b, err := yaml.Marshal(info)
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	}
	return nil
}
