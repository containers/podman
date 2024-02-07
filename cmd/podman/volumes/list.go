package volumes

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/parse"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	volumeLsDescription = `
podman volume ls

List all available volumes. The output of the volumes can be filtered
and the output format can be changed to JSON or a user specified Go template.`
	lsCommand = &cobra.Command{
		Use:               "ls [options]",
		Aliases:           []string{"list"},
		Args:              validate.NoArgs,
		Short:             "List volumes",
		Long:              volumeLsDescription,
		RunE:              list,
		ValidArgsFunction: completion.AutocompleteNone,
	}
)

var (
	// Temporary struct to hold cli values.
	cliOpts = struct {
		Filter []string
		Format string
		Quiet  bool
	}{}
	lsOpts = entities.VolumeListOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: lsCommand,
		Parent:  volumeCmd,
	})
	flags := lsCommand.Flags()

	filterFlagName := "filter"
	flags.StringArrayVarP(&cliOpts.Filter, filterFlagName, "f", []string{}, "Filter volume output")
	_ = lsCommand.RegisterFlagCompletionFunc(filterFlagName, common.AutocompleteVolumeFilters)

	formatFlagName := "format"
	flags.StringVar(&cliOpts.Format, formatFlagName, "{{range .}}{{.Driver}}\t{{.Name}}\n{{end -}}", "Format volume output using Go template")
	_ = lsCommand.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&entities.VolumeListReport{}))

	flags.BoolP("noheading", "n", false, "Do not print headers")
	flags.BoolVarP(&cliOpts.Quiet, "quiet", "q", false, "Print volume output in quiet mode")
}

func list(cmd *cobra.Command, args []string) error {
	var err error
	if cliOpts.Quiet && cmd.Flag("format").Changed {
		return errors.New("quiet and format flags cannot be used together")
	}
	if len(cliOpts.Filter) > 0 {
		lsOpts.Filter = make(map[string][]string)
	}
	lsOpts.Filter, err = parse.FilterArgumentsIntoFilters(cliOpts.Filter)
	if err != nil {
		return err
	}

	responses, err := registry.ContainerEngine().VolumeList(context.Background(), lsOpts)
	if err != nil {
		return err
	}

	switch {
	case report.IsJSON(cliOpts.Format):
		return outputJSON(responses)
	case len(responses) < 1:
		return nil
	}
	return outputTemplate(cmd, responses)
}

func outputTemplate(cmd *cobra.Command, responses []*entities.VolumeListReport) error {
	noHeading, _ := cmd.Flags().GetBool("noheading")
	headers := report.Headers(entities.VolumeListReport{}, map[string]string{
		"Name": "VOLUME NAME",
	})

	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()

	var err error
	switch {
	case cmd.Flag("format").Changed:
		rpt, err = rpt.Parse(report.OriginUser, cliOpts.Format)
	case cliOpts.Quiet:
		rpt, err = rpt.Parse(report.OriginUser, "{{.Name}}\n")
	default:
		rpt, err = rpt.Parse(report.OriginPodman, cliOpts.Format)
	}
	if err != nil {
		return err
	}

	if (rpt.RenderHeaders) && !noHeading {
		if err := rpt.Execute(headers); err != nil {
			return fmt.Errorf("failed to write report column headers: %w", err)
		}
	}
	return rpt.Execute(responses)
}

func outputJSON(vols []*entities.VolumeListReport) error {
	b, err := json.MarshalIndent(vols, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
