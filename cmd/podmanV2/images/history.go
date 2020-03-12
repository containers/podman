package images

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"text/template"

	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/cmd/podmanV2/report"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	long = `Displays the history of an image.

  The information can be printed out in an easy to read, or user specified format, and can be truncated.`

	// podman _history_
	historyCmd = &cobra.Command{
		Use:               "history [flags] IMAGE",
		Short:             "Show history of a specified image",
		Long:              long,
		Example:           "podman history quay.io/fedora/fedora",
		Args:              cobra.ExactArgs(1),
		PersistentPreRunE: preRunE,
		RunE:              history,
	}
)

var cmdFlags = struct {
	Human   bool
	NoTrunc bool
	Quiet   bool
	Format  string
}{}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: historyCmd,
	})

	historyCmd.SetHelpTemplate(registry.HelpTemplate())
	historyCmd.SetUsageTemplate(registry.UsageTemplate())
	flags := historyCmd.Flags()
	flags.StringVar(&cmdFlags.Format, "format", "", "Change the output to JSON or a Go template")
	flags.BoolVarP(&cmdFlags.Human, "human", "H", true, "Display sizes and dates in human readable format")
	flags.BoolVar(&cmdFlags.NoTrunc, "no-trunc", false, "Do not truncate the output")
	flags.BoolVar(&cmdFlags.NoTrunc, "notruncate", false, "Do not truncate the output")
	flags.BoolVarP(&cmdFlags.Quiet, "quiet", "q", false, "Display the numeric IDs only")
}

func history(cmd *cobra.Command, args []string) error {
	results, err := registry.ImageEngine().History(context.Background(), args[0], entities.ImageHistoryOptions{})
	if err != nil {
		return err
	}

	row := "{{slice $x.ID 0 12}}\t{{toRFC3339 $x.Created}}\t{{ellipsis $x.CreatedBy 45}}\t{{$x.Size}}\t{{$x.Comment}}\n"
	if cmdFlags.Human {
		row = "{{slice $x.ID 0 12}}\t{{toHumanDuration $x.Created}}\t{{ellipsis $x.CreatedBy 45}}\t{{toHumanSize $x.Size}}\t{{$x.Comment}}\n"
	}
	format := "{{range $y, $x := . }}" + row + "{{end}}"

	tmpl := template.Must(template.New("report").Funcs(report.PodmanTemplateFuncs()).Parse(format))
	w := tabwriter.NewWriter(os.Stdout, 8, 2, 2, ' ', 0)

	_, _ = w.Write(report.ReportHeader("id", "created", "created by", "size", "comment"))
	err = tmpl.Execute(w, results.Layers)
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrapf(err, "Failed to print report"))
	}
	w.Flush()
	return nil
}
