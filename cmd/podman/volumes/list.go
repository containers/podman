package volumes

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"text/template"

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
		Use:     "ls",
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
	var w io.Writer = os.Stdout
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
	if cliOpts.Format == "json" {
		return outputJSON(responses)
	}

	if len(responses) < 1 {
		return nil
	}
	// "\t" from the command line is not being recognized as a tab
	// replacing the string "\t" to a tab character if the user passes in "\t"
	cliOpts.Format = strings.Replace(cliOpts.Format, `\t`, "\t", -1)
	if cliOpts.Quiet {
		cliOpts.Format = "{{.Name}}\n"
	}
	headers := "DRIVER\tVOLUME NAME\n"
	row := cliOpts.Format
	if !strings.HasSuffix(cliOpts.Format, "\n") {
		row += "\n"
	}
	format := "{{range . }}" + row + "{{end}}"
	if !cliOpts.Quiet && !cmd.Flag("format").Changed {
		w = tabwriter.NewWriter(os.Stdout, 12, 2, 2, ' ', 0)
		format = headers + format
	}
	tmpl, err := template.New("listVolume").Parse(format)
	if err != nil {
		return err
	}
	if err := tmpl.Execute(w, responses); err != nil {
		return err
	}
	if flusher, ok := w.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}

func outputJSON(vols []*entities.VolumeListReport) error {
	b, err := json.MarshalIndent(vols, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
