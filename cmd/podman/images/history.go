package images

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"text/template"
	"time"
	"unicode"

	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	long = `Displays the history of an image.

  The information can be printed out in an easy to read, or user specified format, and can be truncated.`

	// podman _history_
	historyCmd = &cobra.Command{
		Use:     "history [flags] IMAGE",
		Short:   "Show history of a specified image",
		Long:    long,
		Example: "podman history quay.io/fedora/fedora",
		Args:    cobra.ExactArgs(1),
		RunE:    history,
	}

	imageHistoryCmd = &cobra.Command{
		Args:    historyCmd.Args,
		Use:     historyCmd.Use,
		Short:   historyCmd.Short,
		Long:    historyCmd.Long,
		RunE:    historyCmd.RunE,
		Example: `podman image history imageID`,
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
	historyFlags(historyCmd.Flags())

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: imageHistoryCmd,
		Parent:  imageCmd,
	})
	historyFlags(imageHistoryCmd.Flags())
}

func historyFlags(flags *pflag.FlagSet) {
	flags.StringVar(&opts.format, "format", "", "Change the output to JSON or a Go template")
	flags.BoolVarP(&opts.human, "human", "H", true, "Display sizes and dates in human readable format")
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
				layers[i].Created = l.Created.Format(time.RFC3339)
			}
			enc := json.NewEncoder(os.Stdout)
			err = enc.Encode(layers)
		}
		return err
	}
	hr := make([]historyreporter, 0, len(results.Layers))
	for _, l := range results.Layers {
		hr = append(hr, historyreporter{l})
	}
	// Defaults
	hdr := "ID\tCREATED\tCREATED BY\tSIZE\tCOMMENT\n"
	row := "{{.ID}}\t{{.Created}}\t{{.CreatedBy}}\t{{.Size}}\t{{.Comment}}\n"

	switch {
	case len(opts.format) > 0:
		hdr = ""
		row = opts.format
		if !strings.HasSuffix(opts.format, "\n") {
			row += "\n"
		}
	case opts.quiet:
		hdr = ""
		row = "{{.ID}}\n"
	case opts.human:
		row = "{{.ID}}\t{{.Created}}\t{{.CreatedBy}}\t{{.Size}}\t{{.Comment}}\n"
	case opts.noTrunc:
		row = "{{.ID}}\t{{.Created}}\t{{.CreatedBy}}\t{{.Size}}\t{{.Comment}}\n"
	}
	format := hdr + "{{range . }}" + row + "{{end}}"

	tmpl, err := template.New("report").Parse(format)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 8, 2, 2, ' ', 0)
	err = tmpl.Execute(w, hr)
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrapf(err, "Failed to print report"))
	}
	w.Flush()
	return nil
}

type historyreporter struct {
	entities.ImageHistoryLayer
}

func (h historyreporter) Created() string {
	if opts.human {
		return units.HumanDuration(time.Since(h.ImageHistoryLayer.Created)) + " ago"
	}
	return h.ImageHistoryLayer.Created.Format(time.RFC3339)
}

func (h historyreporter) Size() string {
	s := units.HumanSizeWithPrecision(float64(h.ImageHistoryLayer.Size), 3)
	i := strings.LastIndexFunc(s, unicode.IsNumber)
	return s[:i+1] + " " + s[i+1:]
}

func (h historyreporter) CreatedBy() string {
	if len(h.ImageHistoryLayer.CreatedBy) > 45 {
		return h.ImageHistoryLayer.CreatedBy[:45-3] + "..."
	}
	return h.ImageHistoryLayer.CreatedBy
}

func (h historyreporter) ID() string {
	if !opts.noTrunc && len(h.ImageHistoryLayer.ID) >= 12 {
		return h.ImageHistoryLayer.ID[0:12]
	}
	return h.ImageHistoryLayer.ID
}
