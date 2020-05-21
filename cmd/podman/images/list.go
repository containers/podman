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
	"unicode"

	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/docker/go-units"
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
		Use:     "list [FLAGS] [IMAGE]",
		Aliases: []string{"ls"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "List images in local storage",
		Long:    "Lists images previously pulled to the system or created on the system.",
		RunE:    images,
		Example: `podman image list --format json
  podman image list --sort repository --format "table {{.ID}} {{.Repository}} {{.Tag}}"
  podman image list --filter dangling=true`,
		DisableFlagsInUseLine: true,
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

	if len(args) > 0 {
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

	switch {
	case listFlag.quiet:
		return writeId(summaries)
	case cmd.Flag("format").Changed && listFlag.format == "json":
		return writeJSON(summaries)
	default:
		return writeTemplate(summaries)
	}
}

func writeId(imageS []*entities.ImageSummary) error {
	var ids = map[string]struct{}{}
	for _, e := range imageS {
		i := "sha256:" + e.ID
		if !listFlag.noTrunc {
			i = fmt.Sprintf("%12.12s", e.ID)
		}
		ids[i] = struct{}{}
	}
	for k := range ids {
		fmt.Fprint(os.Stdout, k+"\n")
	}
	return nil
}

func writeJSON(imageS []*entities.ImageSummary) error {
	type image struct {
		entities.ImageSummary
		Created   string
		CreatedAt string
	}

	imgs := make([]image, 0, len(imageS))
	for _, e := range imageS {
		var h image
		h.ImageSummary = *e
		h.Created = units.HumanDuration(time.Since(e.Created)) + " ago"
		h.CreatedAt = e.Created.Format(time.RFC3339Nano)
		h.RepoTags = nil

		imgs = append(imgs, h)
	}

	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(imgs)
}

func writeTemplate(imageS []*entities.ImageSummary) error {
	var (
		hdr, row string
	)
	imgs := make([]imageReporter, 0, len(imageS))
	for _, e := range imageS {
		var h imageReporter
		if len(e.RepoTags) > 0 {
			for _, tag := range e.RepoTags {
				h.ImageSummary = *e
				h.Repository, h.Tag = tokenRepoTag(tag)
				imgs = append(imgs, h)
			}
		} else {
			h.ImageSummary = *e
			h.Repository = "<none>"
			imgs = append(imgs, h)
		}
		listFlag.readOnly = e.IsReadOnly()
	}

	sort.Slice(imgs, sortFunc(listFlag.sort, imgs))

	if len(listFlag.format) < 1 {
		hdr, row = imageListFormat(listFlag)
	} else {
		row = listFlag.format
		if !strings.HasSuffix(row, "\n") {
			row += "\n"
		}
	}
	format := hdr + "{{range . }}" + row + "{{end}}"
	tmpl := template.Must(template.New("list").Parse(format))
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

func sortFunc(key string, data []imageReporter) func(i, j int) bool {
	switch key {
	case "id":
		return func(i, j int) bool {
			return data[i].ID() < data[j].ID()
		}
	case "repository":
		return func(i, j int) bool {
			return data[i].Repository < data[j].Repository
		}
	case "size":
		return func(i, j int) bool {
			return data[i].size() < data[j].size()
		}
	case "tag":
		return func(i, j int) bool {
			return data[i].Tag < data[j].Tag
		}
	default:
		// case "created":
		return func(i, j int) bool {
			return data[i].created().After(data[j].created())
		}
	}
}

func imageListFormat(flags listFlagType) (string, string) {
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
		row += "\t{{.ID}}"
	}

	hdr += "\tCREATED\tSIZE"
	row += "\t{{.Created}}\t{{.Size}}"

	if flags.history {
		hdr += "\tHISTORY"
		row += "\t{{if .History}}{{.History}}{{else}}<none>{{end}}"
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

type imageReporter struct {
	Repository string `json:"repository,omitempty"`
	Tag        string `json:"tag,omitempty"`
	entities.ImageSummary
}

func (i imageReporter) ID() string {
	if !listFlag.noTrunc && len(i.ImageSummary.ID) >= 12 {
		return i.ImageSummary.ID[0:12]
	}
	return "sha256:" + i.ImageSummary.ID
}

func (i imageReporter) Created() string {
	return units.HumanDuration(time.Since(i.ImageSummary.Created)) + " ago"
}

func (i imageReporter) created() time.Time {
	return i.ImageSummary.Created
}

func (i imageReporter) Size() string {
	s := units.HumanSizeWithPrecision(float64(i.ImageSummary.Size), 3)
	j := strings.LastIndexFunc(s, unicode.IsNumber)
	return s[:j+1] + " " + s[j+1:]
}

func (i imageReporter) History() string {
	return strings.Join(i.ImageSummary.History, ", ")
}

func (i imageReporter) CreatedAt() string {
	return i.ImageSummary.Created.String()
}

func (i imageReporter) CreatedSince() string {
	return i.Created()
}

func (i imageReporter) CreatedTime() string {
	return i.CreatedAt()
}

func (i imageReporter) size() int64 {
	return i.ImageSummary.Size
}
