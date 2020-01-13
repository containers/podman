package main

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/containers/buildah/pkg/formats"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/docker/go-units"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type imagesTemplateParams struct {
	Repository  string
	Tag         string
	ID          string
	Digest      digest.Digest
	Digests     []digest.Digest
	Created     string
	CreatedTime time.Time
	Size        string
	ReadOnly    bool
	History     string
}

type imagesJSONParams struct {
	ID       string          `json:"id"`
	Name     []string        `json:"names"`
	Digest   digest.Digest   `json:"digest"`
	Digests  []digest.Digest `json:"digests"`
	Created  time.Time       `json:"created"`
	Size     *uint64         `json:"size"`
	ReadOnly bool            `json:"readonly"`
	History  []string        `json:"history"`
}

type imagesOptions struct {
	quiet        bool
	noHeading    bool
	noTrunc      bool
	digests      bool
	format       string
	outputformat string
	sort         string
	all          bool
	history      bool
}

// Type declaration and functions for sorting the images output
type imagesSorted []imagesTemplateParams

func (a imagesSorted) Len() int      { return len(a) }
func (a imagesSorted) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

type imagesSortedCreated struct{ imagesSorted }

func (a imagesSortedCreated) Less(i, j int) bool {
	return a.imagesSorted[i].CreatedTime.After(a.imagesSorted[j].CreatedTime)
}

type imagesSortedID struct{ imagesSorted }

func (a imagesSortedID) Less(i, j int) bool { return a.imagesSorted[i].ID < a.imagesSorted[j].ID }

type imagesSortedTag struct{ imagesSorted }

func (a imagesSortedTag) Less(i, j int) bool { return a.imagesSorted[i].Tag < a.imagesSorted[j].Tag }

type imagesSortedRepository struct{ imagesSorted }

func (a imagesSortedRepository) Less(i, j int) bool {
	return a.imagesSorted[i].Repository < a.imagesSorted[j].Repository
}

type imagesSortedSize struct{ imagesSorted }

func (a imagesSortedSize) Less(i, j int) bool {
	size1, _ := units.FromHumanSize(a.imagesSorted[i].Size)
	size2, _ := units.FromHumanSize(a.imagesSorted[j].Size)
	return size1 < size2
}

var (
	imagesCommand     cliconfig.ImagesValues
	imagesDescription = "Lists images previously pulled to the system or created on the system."

	_imagesCommand = cobra.Command{
		Use:   "images [flags] [IMAGE]",
		Short: "List images in local storage",
		Long:  imagesDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			imagesCommand.InputArgs = args
			imagesCommand.GlobalFlags = MainGlobalOpts
			imagesCommand.Remote = remoteclient
			return imagesCmd(&imagesCommand)
		},
		Example: `podman images --format json
  podman images --sort repository --format "table {{.ID}} {{.Repository}} {{.Tag}}"
  podman images --filter dangling=true`,
	}
)

func imagesInit(command *cliconfig.ImagesValues) {
	command.SetHelpTemplate(HelpTemplate())
	command.SetUsageTemplate(UsageTemplate())

	flags := command.Flags()
	flags.BoolVarP(&command.All, "all", "a", false, "Show all images (default hides intermediate images)")
	flags.BoolVar(&command.Digests, "digests", false, "Show digests")
	flags.StringSliceVarP(&command.Filter, "filter", "f", []string{}, "Filter output based on conditions provided (default [])")
	flags.StringVar(&command.Format, "format", "", "Change the output format to JSON or a Go template")
	flags.BoolVarP(&command.Noheading, "noheading", "n", false, "Do not print column headings")
	// TODO Need to learn how to deal with second name being a string instead of a char.
	// This needs to be "no-trunc, notruncate"
	flags.BoolVar(&command.NoTrunc, "no-trunc", false, "Do not truncate output")
	flags.BoolVarP(&command.Quiet, "quiet", "q", false, "Display only image IDs")
	flags.StringVar(&command.Sort, "sort", "created", "Sort by created, id, repository, size, or tag")
	flags.BoolVarP(&command.History, "history", "", false, "Display the image name history")

}

func init() {
	imagesCommand.Command = &_imagesCommand
	imagesInit(&imagesCommand)
}

func imagesCmd(c *cliconfig.ImagesValues) error {
	var (
		image string
	)

	ctx := getContext()
	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "Could not get runtime")
	}
	defer runtime.DeferredShutdown(false)
	if len(c.InputArgs) == 1 {
		image = c.InputArgs[0]
	}
	if len(c.InputArgs) > 1 {
		return errors.New("'podman images' requires at most 1 argument")
	}
	if len(c.Filter) > 0 && image != "" {
		return errors.New("can not specify an image and a filter")
	}
	filters := c.Filter
	if len(filters) < 1 {
		filters = append(filters, fmt.Sprintf("reference=%s", image))
	}

	opts := imagesOptions{
		quiet:     c.Quiet,
		noHeading: c.Noheading,
		noTrunc:   c.NoTrunc,
		digests:   c.Digests,
		format:    c.Format,
		sort:      c.Sort,
		all:       c.All,
		history:   c.History,
	}

	opts.outputformat = opts.setOutputFormat()
	filteredImages, err := runtime.GetFilteredImages(filters, false)
	if err != nil {
		return errors.Wrapf(err, "unable to get images")
	}

	for _, image := range filteredImages {
		if image.IsReadOnly() {
			opts.outputformat += "{{.ReadOnly}}\t"
			break
		}
	}
	return generateImagesOutput(ctx, filteredImages, opts)
}

func (i imagesOptions) setOutputFormat() string {
	if i.format != "" {
		// "\t" from the command line is not being recognized as a tab
		// replacing the string "\t" to a tab character if the user passes in "\t"
		return strings.Replace(i.format, `\t`, "\t", -1)
	}
	if i.quiet {
		return formats.IDString
	}
	format := "table {{.Repository}}\t{{if .Tag}}{{.Tag}}{{else}}<none>{{end}}\t"
	if i.noHeading {
		format = "{{.Repository}}\t{{if .Tag}}{{.Tag}}{{else}}<none>{{end}}\t"
	}
	if i.digests {
		format += "{{.Digest}}\t"
	}
	format += "{{.ID}}\t{{.Created}}\t{{.Size}}\t"
	if i.history {
		format += "{{if .History}}{{.History}}{{else}}<none>{{end}}\t"
	}
	return format
}

// imagesToGeneric creates an empty array of interfaces for output
func imagesToGeneric(templParams []imagesTemplateParams, jsonParams []imagesJSONParams) []interface{} {
	genericParams := []interface{}{}
	if len(templParams) > 0 {
		for _, v := range templParams {
			genericParams = append(genericParams, interface{}(v))
		}
		return genericParams
	}
	for _, v := range jsonParams {
		genericParams = append(genericParams, interface{}(v))
	}
	return genericParams
}

func sortImagesOutput(sortBy string, imagesOutput imagesSorted) imagesSorted {
	switch sortBy {
	case "id":
		sort.Sort(imagesSortedID{imagesOutput})
	case "size":
		sort.Sort(imagesSortedSize{imagesOutput})
	case "tag":
		sort.Sort(imagesSortedTag{imagesOutput})
	case "repository":
		sort.Sort(imagesSortedRepository{imagesOutput})
	default:
		// default is created time
		sort.Sort(imagesSortedCreated{imagesOutput})
	}
	return imagesOutput
}

// getImagesTemplateOutput returns the images information to be printed in human readable format
func getImagesTemplateOutput(ctx context.Context, images []*adapter.ContainerImage, opts imagesOptions) imagesSorted {
	var imagesOutput imagesSorted
	for _, img := range images {
		// If all is false and the image doesn't have a name, check to see if the top layer of the image is a parent
		// to another image's top layer. If it is, then it is an intermediate image so don't print out if the --all flag
		// is not set.
		isParent, err := img.IsParent(ctx)
		if err != nil {
			logrus.Errorf("error checking if image is a parent %q: %v", img.ID(), err)
		}
		if !opts.all && len(img.Names()) == 0 && isParent {
			continue
		}
		createdTime := img.Created()

		imageID := "sha256:" + img.ID()
		if !opts.noTrunc {
			imageID = shortID(img.ID())
		}

		// get all specified repo:tag and repo@digest pairs and print them separately
		repopairs, err := image.ReposToMap(img.Names())
		if err != nil {
			logrus.Errorf("error finding tag/digest for %s", img.ID())
		}
	outer:
		for repo, tags := range repopairs {
			for _, tag := range tags {
				size, err := img.Size(ctx)
				var sizeStr string
				if err != nil {
					sizeStr = err.Error()
				} else {
					sizeStr = units.HumanSizeWithPrecision(float64(*size), 3)
					lastNumIdx := strings.LastIndexFunc(sizeStr, unicode.IsNumber)
					sizeStr = sizeStr[:lastNumIdx+1] + " " + sizeStr[lastNumIdx+1:]
				}
				var imageDigest digest.Digest
				if len(tag) == 71 && strings.HasPrefix(tag, "sha256:") {
					imageDigest = digest.Digest(tag)
					tag = ""
				} else if img.Digest() != "" {
					imageDigest = img.Digest()
				}
				params := imagesTemplateParams{
					Repository:  repo,
					Tag:         tag,
					ID:          imageID,
					Digest:      imageDigest,
					Digests:     img.Digests(),
					CreatedTime: createdTime,
					Created:     units.HumanDuration(time.Since(createdTime)) + " ago",
					Size:        sizeStr,
					ReadOnly:    img.IsReadOnly(),
					History:     strings.Join(img.NamesHistory(), ", "),
				}
				imagesOutput = append(imagesOutput, params)
				if opts.quiet { // Show only one image ID when quiet
					break outer
				}
			}
		}
	}

	// Sort images by created time
	sortImagesOutput(opts.sort, imagesOutput)
	return imagesOutput
}

// getImagesJSONOutput returns the images information in its raw form
func getImagesJSONOutput(ctx context.Context, images []*adapter.ContainerImage) []imagesJSONParams {
	imagesOutput := []imagesJSONParams{}
	for _, img := range images {
		size, err := img.Size(ctx)
		if err != nil {
			size = nil
		}
		params := imagesJSONParams{
			ID:       img.ID(),
			Name:     img.Names(),
			Digest:   img.Digest(),
			Digests:  img.Digests(),
			Created:  img.Created(),
			Size:     size,
			ReadOnly: img.IsReadOnly(),
			History:  img.NamesHistory(),
		}
		imagesOutput = append(imagesOutput, params)
	}
	return imagesOutput
}

// generateImagesOutput generates the images based on the format provided

func generateImagesOutput(ctx context.Context, images []*adapter.ContainerImage, opts imagesOptions) error {
	templateMap := GenImageOutputMap()
	var out formats.Writer

	switch opts.format {
	case formats.JSONString:
		imagesOutput := getImagesJSONOutput(ctx, images)
		out = formats.JSONStructArray{Output: imagesToGeneric([]imagesTemplateParams{}, imagesOutput)}
	default:
		imagesOutput := getImagesTemplateOutput(ctx, images, opts)
		out = formats.StdoutTemplateArray{Output: imagesToGeneric(imagesOutput, []imagesJSONParams{}), Template: opts.outputformat, Fields: templateMap}
	}
	return out.Out()
}

// GenImageOutputMap generates the map used for outputting the images header
// without requiring a populated image. This replaces the previous HeaderMap
// call.
func GenImageOutputMap() map[string]string {
	io := imagesTemplateParams{}
	v := reflect.Indirect(reflect.ValueOf(io))
	values := make(map[string]string)

	for i := 0; i < v.NumField(); i++ {
		key := v.Type().Field(i).Name
		value := key
		if value == "ID" {
			value = "Image" + value
		}

		if value == "ReadOnly" {
			values[key] = "R/O"
			continue
		}
		values[key] = strings.ToUpper(splitCamelCase(value))
	}
	return values
}
