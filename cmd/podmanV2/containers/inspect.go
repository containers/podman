package containers

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	jsoniter "github.com/json-iterator/go"
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
)

var (
	inspectOptions entities.ContainerInspectOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: inspectCmd,
		Parent:  containerCmd,
	})
	flags := inspectCmd.Flags()
	flags.StringVarP(&inspectOptions.Format, "format", "f", "", "Change the output format to a Go template")
	flags.BoolVarP(&inspectOptions.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.BoolVarP(&inspectOptions.Size, "size", "s", false, "Display total file size")
	if registry.IsRemote() {
		_ = flags.MarkHidden("latest")
	}
}

func inspect(cmd *cobra.Command, args []string) error {
	responses, err := registry.ContainerEngine().ContainerInspect(context.Background(), args, inspectOptions)
	if err != nil {
		return err
	}
	if inspectOptions.Format == "" {
		b, err := jsoniter.MarshalIndent(responses, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
	format := inspectOptions.Format
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
