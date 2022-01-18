package connection

import (
	"fmt"
	"os"
	"sort"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/system"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/spf13/cobra"
)

var (
	listCmd = &cobra.Command{
		Use:     "list [options]",
		Aliases: []string{"ls"},
		Args:    validate.NoArgs,
		Short:   "List destination for the Podman service(s)",
		Long:    `List destination information for the Podman service(s) in podman configuration`,
		Example: `podman system connection list
  podman system connection ls
  podman system connection ls --format=json`,
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

	listCmd.Flags().String("format", "", "Custom Go template for printing connections")
	_ = listCmd.RegisterFlagCompletionFunc("format", common.AutocompleteFormat(namedDestination{}))
}

type namedDestination struct {
	Name string
	config.Destination
	Default bool
}

func list(cmd *cobra.Command, _ []string) error {
	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return err
	}

	rows := make([]namedDestination, 0)
	for k, v := range cfg.Engine.ServiceDestinations {
		def := false
		if k == cfg.Engine.ActiveService {
			def = true
		}

		r := namedDestination{
			Name: k,
			Destination: config.Destination{
				Identity: v.Identity,
				URI:      v.URI,
			},
			Default: def,
		}
		rows = append(rows, r)
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Name < rows[j].Name
	})

	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()

	if report.IsJSON(cmd.Flag("format").Value.String()) {
		buf, err := registry.JSONLibrary().MarshalIndent(rows, "", "    ")
		if err == nil {
			fmt.Println(string(buf))
		}
		return err
	}

	if cmd.Flag("format").Changed {
		rpt, err = rpt.Parse(report.OriginUser, cmd.Flag("format").Value.String())
	} else {
		rpt, err = rpt.Parse(report.OriginPodman,
			"{{range .}}{{.Name}}\t{{.URI}}\t{{.Identity}}\t{{.Default}}\n{{end -}}")
	}
	if err != nil {
		return err
	}

	if rpt.RenderHeaders {
		err = rpt.Execute([]map[string]string{{
			"Default":  "Default",
			"Identity": "Identity",
			"Name":     "Name",
			"URI":      "URI",
		}})
		if err != nil {
			return err
		}
	}
	return rpt.Execute(rows)
}
