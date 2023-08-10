package farm

import (
	"fmt"
	"os"
	"sort"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/spf13/cobra"
)

var (
	farmLsDescription = `podman farm ls

List all available farms. The output of the farms can be filtered
and the output format can be changed to JSON or a user specified Go template.`
	lsCommand = &cobra.Command{
		Use:               "list [options]",
		Aliases:           []string{"ls"},
		Args:              validate.NoArgs,
		Short:             "List all existing farms",
		Long:              farmLsDescription,
		RunE:              list,
		ValidArgsFunction: completion.AutocompleteNone,
	}

	// Temporary struct to hold cli values.
	lsOpts = struct {
		Format string
	}{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: lsCommand,
		Parent:  farmCmd,
	})
	flags := lsCommand.Flags()

	formatFlagName := "format"
	flags.StringVar(&lsOpts.Format, formatFlagName, "", "Format farm output using Go template")
	_ = lsCommand.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&farmOut{}))
}

type farmOut struct {
	Name        string
	Connections []string
	Default     bool
}

func list(cmd *cobra.Command, args []string) error {
	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return err
	}

	format := lsOpts.Format
	if format == "" && len(args) > 0 {
		format = "json"
	}

	rows := make([]farmOut, 0)
	for k, v := range cfg.Farms.List {
		defaultFarm := false
		if k == cfg.Farms.Default {
			defaultFarm = true
		}

		r := farmOut{
			Name:        k,
			Connections: v,
			Default:     defaultFarm,
		}
		rows = append(rows, r)
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Name < rows[j].Name
	})

	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()

	if report.IsJSON(format) {
		buf, err := registry.JSONLibrary().MarshalIndent(rows, "", "    ")
		if err == nil {
			fmt.Println(string(buf))
		}
		return err
	}

	if format != "" {
		rpt, err = rpt.Parse(report.OriginUser, format)
	} else {
		rpt, err = rpt.Parse(report.OriginPodman,
			"{{range .}}{{.Name}}\t{{.Connections}}\t{{.Default}}\n{{end -}}")
	}
	if err != nil {
		return err
	}

	if rpt.RenderHeaders {
		err = rpt.Execute([]map[string]string{{
			"Default":     "Default",
			"Connections": "Connections",
			"Name":        "Name",
		}})
		if err != nil {
			return err
		}
	}

	return rpt.Execute(rows)
}
