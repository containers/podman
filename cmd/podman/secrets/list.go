package secrets

import (
	"context"
	"html/template"
	"os"
	"strings"
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
	cliOpts = listFlagType{}
)

type listFlagType struct {
	format    string
	noHeading bool
	filters   []string
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: lsCmd,
		Parent:  secretCmd,
	})

	flags := lsCmd.Flags()
	formatFlagName := "format"
	flags.StringVar(&cliOpts.format, formatFlagName, "{{.ID}}\t{{.Name}}\t{{.Driver}}\t{{.CreatedAt}}\t{{.UpdatedAt}}\t\n", "Format volume output using Go template")
	_ = lsCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(entities.SecretInfoReport{}))

	flags.BoolVar(&cliOpts.noHeading, "noheading", false, "Do not print headers")

	flags.StringSliceVarP(&cliOpts.filters, "filter", "f", []string{}, "Filter output based on conditions given")
	_ = lsCmd.RegisterFlagCompletionFunc("filter", common.AutocompleteSecretFilters)
}

func ls(cmd *cobra.Command, args []string) error {
	listOpts := entities.SecretListOptions{}

	if len(cliOpts.filters) > 0 {
		listOpts.Filters = make(map[string][]string)
	}
	for _, f := range cliOpts.filters {
		filterSplit := strings.SplitN(f, "=", 2)
		if len(filterSplit) < 2 {
			return errors.Errorf("filter input must be in the form of filter=value: %s is invalid", f)
		}
		listOpts.Filters[filterSplit[0]] = append(listOpts.Filters[filterSplit[0]], filterSplit[1])
	}

	responses, err := registry.ContainerEngine().SecretList(context.Background(), listOpts)
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

	row := report.NormalizeFormat(cliOpts.format)
	format := parse.EnforceRange(row)

	tmpl, err := template.New("list secret").Parse(format)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 12, 2, 2, ' ', 0)
	defer w.Flush()

	if cmd.Flags().Changed("format") && !parse.HasTable(cliOpts.format) {
		cliOpts.noHeading = true
	}

	if !cliOpts.noHeading {
		if err := tmpl.Execute(w, headers); err != nil {
			return errors.Wrapf(err, "failed to write report column headers")
		}
	}
	return tmpl.Execute(w, responses)
}
