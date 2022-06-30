package images

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/report"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/docker/go-units"
	"github.com/spf13/cobra"
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
	imageListCmd = &cobra.Command{
		Use:               "list [options] [IMAGE]",
		Aliases:           []string{"ls"},
		Args:              cobra.MaximumNArgs(1),
		Short:             "List images in local storage",
		Long:              "Lists images previously pulled to the system or created on the system.",
		RunE:              images,
		ValidArgsFunction: common.AutocompleteImages,
		Example: `podman image list --format json
  podman image list --sort repository --format "table {{.ID}} {{.Repository}} {{.Tag}}"
  podman image list --filter dangling=true`,
	}

	imagesCmd = &cobra.Command{
		Use:               "images [options] [IMAGE]",
		Args:              imageListCmd.Args,
		Short:             imageListCmd.Short,
		Long:              imageListCmd.Long,
		RunE:              imageListCmd.RunE,
		ValidArgsFunction: imageListCmd.ValidArgsFunction,
		Example: `podman images --format json
  podman images --sort repository --format "table {{.ID}} {{.Repository}} {{.Tag}}"
  podman images --filter dangling=true`,
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
		Command: imageListCmd,
		Parent:  imageCmd,
	})
	imageListFlagSet(imageListCmd)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: imagesCmd,
	})
	imageListFlagSet(imagesCmd)
}

func imageListFlagSet(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVarP(&listOptions.All, "all", "a", false, "Show all images (default hides intermediate images)")

	filterFlagName := "filter"
	flags.StringSliceVarP(&listOptions.Filter, filterFlagName, "f", []string{}, "Filter output based on conditions provided (default [])")
	_ = cmd.RegisterFlagCompletionFunc(filterFlagName, common.AutocompleteImageFilters)

	formatFlagName := "format"
	flags.StringVar(&listFlag.format, formatFlagName, "", "Change the output format to JSON or a Go template")
	_ = cmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&imageReporter{}))

	flags.BoolVar(&listFlag.digests, "digests", false, "Show digests")
	flags.BoolVarP(&listFlag.noHeading, "noheading", "n", false, "Do not print column headings")
	flags.BoolVar(&listFlag.noTrunc, "no-trunc", false, "Do not truncate output")
	flags.BoolVarP(&listFlag.quiet, "quiet", "q", false, "Display only image IDs")

	sortFlagName := "sort"
	flags.StringVar(&listFlag.sort, sortFlagName, "created", "Sort by "+sortFields.String())
	_ = cmd.RegisterFlagCompletionFunc(sortFlagName, completion.AutocompleteNone)

	flags.BoolVarP(&listFlag.history, "history", "", false, "Display the image name history")
}

func images(cmd *cobra.Command, args []string) error {
	if len(listOptions.Filter) > 0 && len(args) > 0 {
		return errors.New("cannot specify an image and a filter(s)")
	}

	if len(args) > 0 {
		listOptions.Filter = append(listOptions.Filter, "reference="+args[0])
	}

	if cmd.Flags().Changed("sort") && !sortFields.Contains(listFlag.sort) {
		return fmt.Errorf("\"%s\" is not a valid field for sorting. Choose from: %s",
			listFlag.sort, sortFields.String())
	}

	summaries, err := registry.ImageEngine().List(registry.GetContext(), listOptions)
	if err != nil {
		return err
	}

	imgs, err := sortImages(summaries)
	if err != nil {
		return err
	}
	switch {
	case report.IsJSON(listFlag.format):
		return writeJSON(imgs)
	case listFlag.quiet:
		return writeID(imgs)
	default:
		if cmd.Flags().Changed("format") && !report.HasTable(listFlag.format) {
			listFlag.noHeading = true
		}
		return writeTemplate(cmd, imgs)
	}
}

func writeID(imgs []imageReporter) error {
	lookup := make(map[string]struct{}, len(imgs))
	ids := make([]string, 0)

	for _, e := range imgs {
		if _, found := lookup[e.ID()]; !found {
			lookup[e.ID()] = struct{}{}
			ids = append(ids, e.ID())
		}
	}
	for _, k := range ids {
		fmt.Println(k)
	}
	return nil
}

func writeJSON(images []imageReporter) error {
	type image struct {
		entities.ImageSummary
		Created   int64
		CreatedAt string
	}

	imgs := make([]image, 0, len(images))
	for _, e := range images {
		var h image
		h.ImageSummary = e.ImageSummary
		h.Created = e.ImageSummary.Created
		h.CreatedAt = e.created().Format(time.RFC3339Nano)
		h.RepoTags = nil

		imgs = append(imgs, h)
	}

	prettyJSON, err := json.MarshalIndent(imgs, "", "    ")
	if err != nil {
		return err
	}
	fmt.Println(string(prettyJSON))
	return nil
}

func writeTemplate(cmd *cobra.Command, imgs []imageReporter) error {
	hdrs := report.Headers(imageReporter{}, map[string]string{
		"ID":       "IMAGE ID",
		"ReadOnly": "R/O",
	})

	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()

	var err error
	if cmd.Flags().Changed("format") {
		rpt, err = rpt.Parse(report.OriginUser, cmd.Flag("format").Value.String())
	} else {
		rpt, err = rpt.Parse(report.OriginPodman, lsFormatFromFlags(listFlag))
	}
	if err != nil {
		return err
	}

	if rpt.RenderHeaders && !listFlag.noHeading {
		if err := rpt.Execute(hdrs); err != nil {
			return err
		}
	}
	return rpt.Execute(imgs)
}

func sortImages(imageS []*entities.ImageSummary) ([]imageReporter, error) {
	imgs := make([]imageReporter, 0, len(imageS))
	var err error
	for _, e := range imageS {
		var h imageReporter
		if len(e.RepoTags) > 0 {
			tagged := []imageReporter{}
			untagged := []imageReporter{}
			for _, tag := range e.RepoTags {
				h.ImageSummary = *e
				h.Repository, h.Tag, err = tokenRepoTag(tag)
				if err != nil {
					return nil, fmt.Errorf("error parsing repository tag: %q: %w", tag, err)
				}
				if h.Tag == "<none>" {
					untagged = append(untagged, h)
				} else {
					tagged = append(tagged, h)
				}
			}
			// Note: we only want to display "<none>" if we
			// couldn't find any tagged name in RepoTags.
			if len(tagged) > 0 {
				imgs = append(imgs, tagged...)
			} else {
				imgs = append(imgs, untagged[0])
			}
		} else {
			h.ImageSummary = *e
			h.Repository = "<none>"
			h.Tag = "<none>"
			imgs = append(imgs, h)
		}
		listFlag.readOnly = e.IsReadOnly()
	}

	sort.Slice(imgs, sortFunc(listFlag.sort, imgs))
	return imgs, err
}

func tokenRepoTag(ref string) (string, string, error) {
	if ref == "<none>:<none>" {
		return "<none>", "<none>", nil
	}

	repo, err := reference.Parse(ref)
	if err != nil {
		return "<none>", "<none>", err
	}

	named, ok := repo.(reference.Named)
	if !ok {
		return ref, "<none>", nil
	}
	name := named.Name()
	if name == "" {
		name = "<none>"
	}

	tagged, ok := repo.(reference.Tagged)
	if !ok {
		return name, "<none>", nil
	}
	tag := tagged.Tag()
	if tag == "" {
		tag = "<none>"
	}

	return name, tag, nil
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

func lsFormatFromFlags(flags listFlagType) string {
	row := []string{
		"{{if .Repository}}{{.Repository}}{{else}}<none>{{end}}",
		"{{if .Tag}}{{.Tag}}{{else}}<none>{{end}}",
	}

	if flags.digests {
		row = append(row, "{{.Digest}}")
	}

	row = append(row, "{{.ID}}", "{{.Created}}", "{{.Size}}")

	if flags.history {
		row = append(row, "{{if .History}}{{.History}}{{else}}<none>{{end}}")
	}

	if flags.readOnly {
		row = append(row, "{{.ReadOnly}}")
	}

	return "{{range . }}" + strings.Join(row, "\t") + "\n{{end -}}"
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
	return units.HumanDuration(time.Since(i.created())) + " ago"
}

func (i imageReporter) created() time.Time {
	return time.Unix(i.ImageSummary.Created, 0).UTC()
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
	return i.created().String()
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
