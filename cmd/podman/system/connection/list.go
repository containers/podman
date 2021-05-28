package connection

import (
	"os"
	"text/tabwriter"
	"text/template"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/system"
	"github.com/containers/podman/v3/cmd/podman/validate"
	"github.com/spf13/cobra"
)

var (
	listCmd = &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    validate.NoArgs,
		Short:   "List destination for the Podman service(s)",
		Long:    `List destination information for the Podman service(s) in podman configuration`,
		Example: `podman system connection list
  podman system connection ls`,
		ValidArgsFunction: completion.AutocompleteNone,
		RunE:              list,
		TraverseChildren:  false,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: listCmd,
		Parent:  system.ConnectionCmd,
	})
}

type namedDestination struct {
	Name string
	config.Destination
}

func list(_ *cobra.Command, _ []string) error {
	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return err
	}

	if len(cfg.Engine.ServiceDestinations) == 0 {
		return nil
	}

	hdrs := []map[string]string{{
		"Identity": "Identity",
		"Name":     "Name",
		"URI":      "URI",
	}}

	rows := make([]namedDestination, 0)
	for k, v := range cfg.Engine.ServiceDestinations {
		if k == cfg.Engine.ActiveService {
			k += "*"
		}

		r := namedDestination{
			Name: k,
			Destination: config.Destination{
				Identity: v.Identity,
				URI:      v.URI,
			},
		}
		rows = append(rows, r)
	}

	// TODO: Allow user to override format
	format := "{{range . }}{{.Name}}\t{{.Identity}}\t{{.URI}}\n{{end}}"
	tmpl, err := template.New("connection").Parse(format)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 8, 2, 2, ' ', 0)
	defer w.Flush()

	_ = tmpl.Execute(w, hdrs)
	return tmpl.Execute(w, rows)
}
