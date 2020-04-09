package images

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"text/template"
	"time"

	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/cmd/podmanV2/report"
	"github.com/containers/libpod/pkg/domain/entities"
	jsoniter "github.com/json-iterator/go"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type listFlagType struct {
	format    string
	history   bool
	noHeading bool
	noTrunc   bool
	quiet     bool
	sort      string
	readOnly  bool
	digests   bool
}

var (
	// Command: podman image _list_
	listCmd = &cobra.Command{
		Use:     "list [flag] [IMAGE]",
		Aliases: []string{"ls"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "List images in local storage",
		Long:    "Lists images previously pulled to the system or created on the system.",
		RunE:    images,
		Example: `podman image list --format json
  podman image list --sort repository --format "table {{.ID}} {{.Repository}} {{.Tag}}"
  podman image list --filter dangling=true`,
	}

	// Options to pull data
	listOptions = entities.ImageListOptions{}

	// Options for presenting data
	listFlag = listFlagType{}

	sortFields = entities.NewStringSet(
		"created",
		"id",
		"repository",
		"size",
		"tag")
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: listCmd,
		Parent:  imageCmd,
	})
	imageListFlagSet(listCmd.Flags())
}

func imageListFlagSet(flags *pflag.FlagSet) {
	flags.BoolVarP(&listOptions.All, "all", "a", false, "Show all images (default hides intermediate images)")
	flags.StringSliceVarP(&listOptions.Filter, "filter", "f", []string{}, "Filter output based on conditions provided (default [])")
	flags.StringVar(&listFlag.format, "format", "", "Change the output format to JSON or a Go template")
	flags.BoolVar(&listFlag.digests, "digests", false, "Show digests")
	flags.BoolVarP(&listFlag.noHeading, "noheading", "n", false, "Do not print column headings")
	flags.BoolVar(&listFlag.noTrunc, "no-trunc", false, "Do not truncate output")
	flags.BoolVar(&listFlag.noTrunc, "notruncate", false, "Do not truncate output")
	flags.BoolVarP(&listFlag.quiet, "quiet", "q", false, "Display only image IDs")
	flags.StringVar(&listFlag.sort, "sort", "created", "Sort by "+sortFields.String())
	flags.BoolVarP(&listFlag.history, "history", "", false, "Display the image name history")
}

func images(cmd *cobra.Command, args []string) error {
	if len(listOptions.Filter) > 0 && len(args) > 0 {
		return errors.New("cannot specify an image and a filter(s)")
	}

	if len(listOptions.Filter) < 1 && len(args) > 0 {
		listOptions.Filter = append(listOptions.Filter, "reference="+args[0])
	}

	if cmd.Flag("sort").Changed && !sortFields.Contains(listFlag.sort) {
		return fmt.Errorf("\"%s\" is not a valid field for sorting. Choose from: %s",
			listFlag.sort, sortFields.String())
	}

	summaries, err := registry.ImageEngine().List(registry.GetContext(), listOptions)
	if err != nil {
		return err
	}

	imageS := summaries
	sort.Slice(imageS, sortFunc(listFlag.sort, imageS))

	if cmd.Flag("format").Changed && listFlag.format == "json" {
		return writeJSON(imageS)
	} else {
		return writeTemplate(imageS, err)
	}
}

func writeJSON(imageS []*entities.ImageSummary) error {
	type image struct {
		entities.ImageSummary
		Created string
	}

	imgs := make([]image, 0, len(imageS))
	for _, e := range imageS {
		var h image
		h.ImageSummary = *e
		h.Created = time.Unix(e.Created, 0).Format(time.RFC3339)
		h.RepoTags = nil

		imgs = append(imgs, h)
	}

	json := jsoniter.ConfigCompatibleWithStandardLibrary
	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(imgs)
}

func writeTemplate(imageS []*entities.ImageSummary, err error) error {
	var (
		hdr, row string
	)
	type image struct {
		entities.ImageSummary
		Repository string `json:"repository,omitempty"`
		Tag        string `json:"tag,omitempty"`
	}

	imgs := make([]image, 0, len(imageS))
	for _, e := range imageS {
		for _, tag := range e.RepoTags {
			var h image
			h.ImageSummary = *e
			h.Repository, h.Tag = tokenRepoTag(tag)
			imgs = append(imgs, h)
		}
		if e.IsReadOnly() {
			listFlag.readOnly = true
		}
	}
	if len(listFlag.format) < 1 {
		hdr, row = imageListFormat(listFlag)
	} else {
		row = listFlag.format
		if !strings.HasSuffix(row, "\n") {
			row += "\n"
		}
	}
	format := hdr + "{{range . }}" + row + "{{end}}"
	tmpl := template.Must(template.New("list").Funcs(report.PodmanTemplateFuncs()).Parse(format))
	w := tabwriter.NewWriter(os.Stdout, 8, 2, 2, ' ', 0)
	defer w.Flush()
	return tmpl.Execute(w, imgs)
}

func tokenRepoTag(tag string) (string, string) {
	tokens := strings.SplitN(tag, ":", 2)
	switch len(tokens) {
	case 0:
		return tag, ""
	case 1:
		return tokens[0], ""
	case 2:
		return tokens[0], tokens[1]
	default:
		return "<N/A>", ""
	}
}

func sortFunc(key string, data []*entities.ImageSummary) func(i, j int) bool {
	switch key {
	case "id":
		return func(i, j int) bool {
			return data[i].ID < data[j].ID
		}
	case "repository":
		return func(i, j int) bool {
			return data[i].RepoTags[0] < data[j].RepoTags[0]
		}
	case "size":
		return func(i, j int) bool {
			return data[i].Size < data[j].Size
		}
	case "tag":
		return func(i, j int) bool {
			return data[i].RepoTags[0] < data[j].RepoTags[0]
		}
	default:
		// case "created":
		return func(i, j int) bool {
			return data[i].Created >= data[j].Created
		}
	}
}

func imageListFormat(flags listFlagType) (string, string) {
	if flags.quiet {
		return "", "{{slice .ID 0 12}}\n"
	}

	// Defaults
	hdr := "REPOSITORY\tTAG"
	row := "{{.Repository}}\t{{if .Tag}}{{.Tag}}{{else}}<none>{{end}}"

	if flags.digests {
		hdr += "\tDIGEST"
		row += "\t{{.Digest}}"
	}

	hdr += "\tIMAGE ID"
	if flags.noTrunc {
		row += "\tsha256:{{.ID}}"
	} else {
		row += "\t{{slice .ID 0 12}}"
	}

	hdr += "\tCREATED\tSIZE"
	row += "\t{{humanDuration .Created}}\t{{humanSize .Size}}"

	if flags.history {
		hdr += "\tHISTORY"
		row += "\t{{if .History}}{{join .History \", \"}}{{else}}<none>{{end}}"
	}

	if flags.readOnly {
		hdr += "\tReadOnly"
		row += "\t{{.ReadOnly}}"
	}

	if flags.noHeading {
		hdr = ""
	} else {
		hdr += "\n"
	}

	row += "\n"
	return hdr, row
}
