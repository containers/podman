package system

import (
	"encoding/json"
	"fmt"
	"os"
	"text/template"

	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var (
	infoDescription = `Display information pertaining to the host, current storage stats, and build of podman.

  Useful for the user and when reporting issues.
`
	infoCommand = &cobra.Command{
		Use:     "info",
		Args:    cobra.NoArgs,
		Long:    infoDescription,
		Short:   "Display podman system information",
		PreRunE: preRunE,
		RunE:    info,
		Example: `podman info`,
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
	flags := infoCommand.Flags()
	flags.BoolVarP(&debug, "debug", "D", false, "Display additional debug information")
	flags.StringVarP(&inFormat, "format", "f", "", "Change the output format to JSON or a Go template")
}

func info(cmd *cobra.Command, args []string) error {
	info, err := registry.ContainerEngine().Info(registry.GetContext())
	if err != nil {
		return err
	}

	if inFormat == "json" {
		b, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
	if !cmd.Flag("format").Changed {
		b, err := yaml.Marshal(info)
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
	tmpl, err := template.New("info").Parse(inFormat)
	if err != nil {
		return err
	}
	err = tmpl.Execute(os.Stdout, info)
	return err
}
