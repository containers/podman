package connection

import (
	"fmt"
	"os"
	"slices"
	"sort"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/system"
	"github.com/containers/podman/v5/cmd/podman/validate"
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
	inspectCmd = &cobra.Command{
		Use:               "inspect [options] [CONTEXT] [CONTEXT...]",
		Short:             "Inspect destination for a Podman service(s)",
		ValidArgsFunction: completion.AutocompleteNone,
		RunE:              inspect,
	}
)

func init() {
	initFlags := func(cmd *cobra.Command) {
		cmd.Flags().StringP("format", "f", "", "Custom Go template for printing connections")
		_ = cmd.RegisterFlagCompletionFunc("format", common.AutocompleteFormat(&config.Connection{}))
		cmd.Flags().BoolP("quiet", "q", false, "Custom Go template for printing connections")
	}

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: listCmd,
		Parent:  system.ContextCmd,
	})
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: listCmd,
		Parent:  system.ConnectionCmd,
	})
	initFlags(listCmd)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: inspectCmd,
		Parent:  system.ContextCmd,
	})
	initFlags(inspectCmd)
}

func list(cmd *cobra.Command, _ []string) error {
	return inspect(cmd, nil)
}

func inspect(cmd *cobra.Command, args []string) error {
	format := cmd.Flag("format").Value.String()
	if format == "" && args != nil {
		format = "json"
	}

	quiet, err := cmd.Flags().GetBool("quiet")
	if err != nil {
		return err
	}

	cons, err := registry.PodmanConfig().ContainersConfDefaultsRO.GetAllConnections()
	if err != nil {
		return err
	}
	rows := make([]config.Connection, 0, len(cons))
	for _, con := range cons {
		if args != nil && !slices.Contains(args, con.Name) {
			continue
		}

		if quiet {
			fmt.Println(con.Name)
			continue
		}

		rows = append(rows, con)
	}

	if quiet {
		return nil
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
			"{{range .}}{{.Name}}\t{{.URI}}\t{{.Identity}}\t{{.Default}}\t{{.ReadWrite}}\n{{end -}}")
	}
	if err != nil {
		return err
	}

	if rpt.RenderHeaders {
		err = rpt.Execute([]map[string]string{{
			"Default":   "Default",
			"Identity":  "Identity",
			"Name":      "Name",
			"URI":       "URI",
			"ReadWrite": "ReadWrite",
		}})
		if err != nil {
			return err
		}
	}
	return rpt.Execute(rows)
}
