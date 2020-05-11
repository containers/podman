package network

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"os"
	"strings"

	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	networkinspectDescription = `Inspect network`
	networkinspectCommand     = &cobra.Command{
		Use:     "inspect NETWORK [NETWORK...] [flags] ",
		Short:   "network inspect",
		Long:    networkinspectDescription,
		RunE:    networkInspect,
		Example: `podman network inspect podman`,
		Args:    cobra.MinimumNArgs(1),
		Annotations: map[string]string{
			registry.ParentNSRequired: "",
		},
	}
)

var (
	networkInspectOptions entities.NetworkInspectOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: networkinspectCommand,
		Parent:  networkCmd,
	})
	flags := networkinspectCommand.Flags()
	flags.StringVarP(&networkInspectOptions.Format, "format", "f", "", "Pretty-print network to JSON or using a Go template")
}

func networkInspect(cmd *cobra.Command, args []string) error {
	responses, err := registry.ContainerEngine().NetworkInspect(registry.Context(), args, entities.NetworkInspectOptions{})
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(responses, "", "  ")
	if err != nil {
		return err
	}
	if strings.ToLower(networkInspectOptions.Format) == "json" || networkInspectOptions.Format == "" {
		fmt.Println(string(b))
	} else {
		var w io.Writer = os.Stdout
		//There can be more than 1 in the inspect output.
		format := "{{range . }}" + networkInspectOptions.Format + "{{end}}"
		tmpl, err := template.New("inspectNetworks").Parse(format)
		if err != nil {
			return err
		}
		if err := tmpl.Execute(w, responses); err != nil {
			return err
		}
		if flusher, ok := w.(interface{ Flush() error }); ok {
			return flusher.Flush()
		}
	}
	return nil
}
