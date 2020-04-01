package containers

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/containers/libpod/cmd/podmanV2/common"
	"github.com/containers/libpod/cmd/podmanV2/registry"

	"github.com/containers/libpod/pkg/domain/entities"
	json "github.com/json-iterator/go"
	"github.com/spf13/cobra"
)

var (
	// podman container _inspect_
	inspectCmd = &cobra.Command{
		Use:     "inspect [flags] CONTAINER",
		Short:   "Display the configuration of a container",
		Long:    `Displays the low-level information on a container identified by name or ID.`,
		PreRunE: preRunE,
		RunE:    inspect,
		Example: `podman container inspect myCtr
  podman container inspect -l --format '{{.Id}} {{.Config.Labels}}'`,
	}
	inspectOpts *entities.InspectOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: inspectCmd,
		Parent:  containerCmd,
	})
	inspectOpts = common.AddInspectFlagSet(inspectCmd)
	flags := inspectCmd.Flags()

	if !registry.IsRemote() {
		flags.BoolVarP(&inspectOpts.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	}

}

func inspect(cmd *cobra.Command, args []string) error {
	responses, err := registry.ContainerEngine().ContainerInspect(context.Background(), args, *inspectOpts)
	if err != nil {
		return err
	}
	if inspectOpts.Format == "" {
		b, err := json.MarshalIndent(responses, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
	format := inspectOpts.Format
	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}
	tmpl, err := template.New("inspect").Parse(format)
	if err != nil {
		return err
	}
	for _, i := range responses {
		if err := tmpl.Execute(os.Stdout, i); err != nil {
			return err
		}
	}
	return nil
}

func Inspect(cmd *cobra.Command, args []string, options *entities.InspectOptions) error {
	inspectOpts = options
	return inspect(cmd, args)
}
