package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/domain/entities"
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
	_ = inspectCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&entities.SecretInfoReport{}))
}

func inspect(cmd *cobra.Command, args []string) error {
	inspected, errs, _ := registry.ContainerEngine().SecretInspect(context.Background(), args)

	// always print valid list
	if len(inspected) == 0 {
		inspected = []*entities.SecretInfoReport{}
	}

	if cmd.Flags().Changed("format") {
		row := report.NormalizeFormat(format)
		formatted := report.EnforceRange(row)

		tmpl, err := report.NewTemplate("inspect").Parse(formatted)
		if err != nil {
			return err
		}

		w, err := report.NewWriterDefault(os.Stdout)
		if err != nil {
			return err
		}
		defer w.Flush()
		if err := tmpl.Execute(w, inspected); err != nil {
			return err
		}
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
		return fmt.Errorf("inspecting secret: %w", errs[0])
	}
	return nil
}
