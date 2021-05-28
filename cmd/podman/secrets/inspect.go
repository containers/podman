package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"text/tabwriter"

	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/parse"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	inspectCmd = &cobra.Command{
		Use:               "inspect [options] SECRET [SECRET...]",
		Short:             "Inspect a secret",
		Long:              "Display detail information on one or more secrets",
		RunE:              inspect,
		Example:           "podman secret inspect MYSECRET",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: common.AutocompleteSecrets,
	}
)

var format string

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: inspectCmd,
		Parent:  secretCmd,
	})
	flags := inspectCmd.Flags()
	formatFlagName := "format"
	flags.StringVar(&format, formatFlagName, "", "Format volume output using Go template")
	_ = inspectCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(entities.SecretInfoReport{}))
}

func inspect(cmd *cobra.Command, args []string) error {
	inspected, errs, _ := registry.ContainerEngine().SecretInspect(context.Background(), args)

	// always print valid list
	if len(inspected) == 0 {
		inspected = []*entities.SecretInfoReport{}
	}

	if cmd.Flags().Changed("format") {
		row := report.NormalizeFormat(format)
		formatted := parse.EnforceRange(row)

		tmpl, err := template.New("inspect secret").Parse(formatted)
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 12, 2, 2, ' ', 0)
		defer w.Flush()
		tmpl.Execute(w, inspected)
	} else {
		buf, err := json.MarshalIndent(inspected, "", "    ")
		if err != nil {
			return err
		}
		fmt.Println(string(buf))
	}

	if len(errs) > 0 {
		if len(errs) > 1 {
			for _, err := range errs[1:] {
				fmt.Fprintf(os.Stderr, "error inspecting secret: %v\n", err)
			}
		}
		return errors.Errorf("error inspecting secret: %v", errs[0])
	}
	return nil
}
