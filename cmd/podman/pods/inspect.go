package pods

import (
	"context"
	"fmt"

	"github.com/containers/buildah/pkg/formats"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/validate"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	inspectOptions = entities.PodInspectOptions{}
)

var (
	inspectDescription = fmt.Sprintf(`Display the configuration for a pod by name or id

	By default, this will render all results in a JSON array.`)

	inspectCmd = &cobra.Command{
		Use:     "inspect [flags] POD [POD...]",
		Short:   "Displays a pod configuration",
		Long:    inspectDescription,
		RunE:    inspect,
		Example: `podman pod inspect podID`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: inspectCmd,
		Parent:  podCmd,
	})
	flags := inspectCmd.Flags()
	flags.StringVarP(&inspectOptions.Format, "format", "f", "json", "Format the output to a Go template or json")
	validate.AddLatestFlag(inspectCmd, &inspectOptions.Latest)
}

func inspect(cmd *cobra.Command, args []string) error {

	if len(args) < 1 && !inspectOptions.Latest {
		return errors.Errorf("you must provide the name or id of a running pod")
	}
	if len(args) > 0 && inspectOptions.Latest {
		return errors.Errorf("--latest and containers cannot be used together")
	}

	if !inspectOptions.Latest {
		inspectOptions.NameOrID = args[0]
	}
	responses, err := registry.ContainerEngine().PodInspect(context.Background(), inspectOptions)
	if err != nil {
		return err
	}
	var data interface{} = responses
	var out formats.Writer = formats.JSONStruct{Output: data}
	if inspectOptions.Format != "json" {
		out = formats.StdoutTemplate{Output: data, Template: inspectOptions.Format}
	}

	return out.Out()
}
