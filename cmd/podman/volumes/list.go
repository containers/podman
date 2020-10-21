package volumes

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/validate"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	volumeLsDescription = `
podman volume ls

List all available volumes. The output of the volumes can be filtered
and the output format can be changed to JSON or a user specified Go template.`
	lsCommand = &cobra.Command{
		Use:     "ls [options]",
		Aliases: []string{"list"},
		Args:    validate.NoArgs,
		Short:   "List volumes",
		Long:    volumeLsDescription,
		RunE:    list,
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
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: lsCommand,
		Parent:  volumeCmd,
	})
	flags := lsCommand.Flags()
	flags.StringSliceVarP(&cliOpts.Filter, "filter", "f", []string{}, "Filter volume output")
	flags.StringVar(&cliOpts.Format, "format", "{{.Driver}}\t{{.Name}}\n", "Format volume output using Go template")
	flags.BoolVarP(&cliOpts.Quiet, "quiet", "q", false, "Print volume output in quiet mode")
}

func list(cmd *cobra.Command, args []string) error {
	if cliOpts.Quiet && cmd.Flag("format").Changed {
		return errors.New("quiet and format flags cannot be used together")
	}
	if len(cliOpts.Filter) > 0 {
		lsOpts.Filter = make(map[string][]string)
	}
	for _, f := range cliOpts.Filter {
		filterSplit := strings.Split(f, "=")
		if len(filterSplit) < 2 {
			return errors.Errorf("filter input must be in the form of filter=value: %s is invalid", f)
		}
		lsOpts.Filter[filterSplit[0]] = append(lsOpts.Filter[filterSplit[0]], filterSplit[1:]...)
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
	headers := report.Headers(entities.VolumeListReport{}, map[string]string{
		"Name": "VOLUME NAME",
	})

	row := report.NormalizeFormat(cliOpts.Format)
	if cliOpts.Quiet {
		row = "{{.Name}}\n"
	}
	row = "{{range . }}" + row + "{{end}}"

	tmpl, err := template.New("list volume").Parse(row)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 12, 2, 2, ' ', 0)
	defer w.Flush()

	if !cliOpts.Quiet && !cmd.Flag("format").Changed {
		if err := tmpl.Execute(w, headers); err != nil {
			return errors.Wrapf(err, "failed to write report column headers")
		}
	}
	return tmpl.Execute(w, responses)
}

func outputJSON(vols []*entities.VolumeListReport) error {
	b, err := json.MarshalIndent(vols, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
