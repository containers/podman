package images

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"text/template"
	"time"

	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/cmd/podmanV2/report"
	"github.com/containers/libpod/pkg/domain/entities"
	jsoniter "github.com/json-iterator/go"
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

	opts = struct {
		human   bool
		noTrunc bool
		quiet   bool
		format  string
	}{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: historyCmd,
	})

	historyCmd.SetHelpTemplate(registry.HelpTemplate())
	historyCmd.SetUsageTemplate(registry.UsageTemplate())

	flags := historyCmd.Flags()
	flags.StringVar(&opts.format, "format", "", "Change the output to JSON or a Go template")
	flags.BoolVarP(&opts.human, "human", "H", false, "Display sizes and dates in human readable format")
	flags.BoolVar(&opts.noTrunc, "no-trunc", false, "Do not truncate the output")
	flags.BoolVar(&opts.noTrunc, "notruncate", false, "Do not truncate the output")
	flags.BoolVarP(&opts.quiet, "quiet", "q", false, "Display the numeric IDs only")
}

func history(cmd *cobra.Command, args []string) error {
	results, err := registry.ImageEngine().History(context.Background(), args[0], entities.ImageHistoryOptions{})
	if err != nil {
		return err
	}

	if opts.format == "json" {
		var err error
		if len(results.Layers) == 0 {
			_, err = fmt.Fprintf(os.Stdout, "[]\n")
		} else {
			// ah-hoc change to "Created": type and format
			type layer struct {
				entities.ImageHistoryLayer
				Created string `json:"Created"`
			}

			layers := make([]layer, len(results.Layers))
			for i, l := range results.Layers {
				layers[i].ImageHistoryLayer = l
				layers[i].Created = time.Unix(l.Created, 0).Format(time.RFC3339)
			}
			json := jsoniter.ConfigCompatibleWithStandardLibrary
			enc := json.NewEncoder(os.Stdout)
			err = enc.Encode(layers)
		}
		return err
	}

	// Defaults
	hdr := "ID\tCREATED\tCREATED BY\tSIZE\tCOMMENT\n"
	row := "{{slice .ID 0 12}}\t{{humanDuration .Created}}\t{{ellipsis .CreatedBy 45}}\t{{.Size}}\t{{.Comment}}\n"

	if len(opts.format) > 0 {
		hdr = ""
		row = opts.format
		if !strings.HasSuffix(opts.format, "\n") {
			row += "\n"
		}
	} else {
		switch {
		case opts.human:
			row = "{{slice .ID 0 12}}\t{{humanDuration .Created}}\t{{ellipsis .CreatedBy 45}}\t{{humanSize .Size}}\t{{.Comment}}\n"
		case opts.noTrunc:
			row = "{{.ID}}\t{{humanDuration .Created}}\t{{.CreatedBy}}\t{{humanSize .Size}}\t{{.Comment}}\n"
		case opts.quiet:
			hdr = ""
			row = "{{.ID}}\n"
		}
	}
	format := hdr + "{{range . }}" + row + "{{end}}"

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
