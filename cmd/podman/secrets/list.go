package secrets

import (
	"context"
	"html/template"
	"os"
	"text/tabwriter"
	"time"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/parse"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/validate"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	lsCmd = &cobra.Command{
		Use:               "ls [options]",
		Aliases:           []string{"list"},
		Short:             "List secrets",
		RunE:              ls,
		Example:           "podman secret ls",
		Args:              validate.NoArgs,
		ValidArgsFunction: completion.AutocompleteNone,
	}
	listFlag = listFlagType{}
)

type listFlagType struct {
	format    string
	noHeading bool
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: lsCmd,
		Parent:  secretCmd,
	})

	flags := lsCmd.Flags()
	formatFlagName := "format"
	flags.StringVar(&listFlag.format, formatFlagName, "{{.ID}}\t{{.Name}}\t{{.Driver}}\t{{.CreatedAt}}\t{{.UpdatedAt}}\t\n", "Format volume output using Go template")
	_ = lsCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(entities.SecretInfoReport{}))
	flags.BoolVar(&listFlag.noHeading, "noheading", false, "Do not print headers")
}

func ls(cmd *cobra.Command, args []string) error {
	responses, err := registry.ContainerEngine().SecretList(context.Background())
	if err != nil {
		return err
	}
	listed := make([]*entities.SecretListReport, 0, len(responses))
	for _, response := range responses {
		listed = append(listed, &entities.SecretListReport{
			ID:        response.ID,
			Name:      response.Spec.Name,
			CreatedAt: units.HumanDuration(time.Since(response.CreatedAt)) + " ago",
			UpdatedAt: units.HumanDuration(time.Since(response.UpdatedAt)) + " ago",
			Driver:    response.Spec.Driver.Name,
		})
	}
	return outputTemplate(cmd, listed)
}

func outputTemplate(cmd *cobra.Command, responses []*entities.SecretListReport) error {
	headers := report.Headers(entities.SecretListReport{}, map[string]string{
		"CreatedAt": "CREATED",
		"UpdatedAt": "UPDATED",
	})

	row := report.NormalizeFormat(listFlag.format)
	format := parse.EnforceRange(row)

	tmpl, err := template.New("list secret").Parse(format)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 12, 2, 2, ' ', 0)
	defer w.Flush()

	if cmd.Flags().Changed("format") && !parse.HasTable(listFlag.format) {
		listFlag.noHeading = true
	}

	if !listFlag.noHeading {
		if err := tmpl.Execute(w, headers); err != nil {
			return errors.Wrapf(err, "failed to write report column headers")
		}
	}
	return tmpl.Execute(w, responses)
}
