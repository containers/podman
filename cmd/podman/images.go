package main

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/formats"
	"github.com/containers/libpod/cmd/podman/imagefilters"
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
	Created     string
	CreatedTime time.Time
	Size        string
}

type imagesJSONParams struct {
	ID      string        `json:"id"`
	Name    []string      `json:"names"`
	Digest  digest.Digest `json:"digest"`
	Created time.Time     `json:"created"`
	Size    *uint64       `json:"size"`
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
	imagesDescription = "lists locally stored images."

	_imagesCommand = cobra.Command{
		Use:   "images",
		Short: "List images in local storage",
		Long:  imagesDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			imagesCommand.InputArgs = args
			imagesCommand.GlobalFlags = MainGlobalOpts
			return imagesCmd(&imagesCommand)
		},
		Example: `podman images --format json
  podman images --sort repository --format "table {{.ID}} {{.Repository}} {{.Tag}}"
  podman images --filter dangling=true`,
	}
)

func init() {
	imagesCommand.Command = &_imagesCommand
	imagesCommand.SetUsageTemplate(UsageTemplate())

	flags := imagesCommand.Flags()
	flags.BoolVarP(&imagesCommand.All, "all", "a", false, "Show all images (default hides intermediate images)")
	flags.BoolVar(&imagesCommand.Digests, "digests", false, "Show digests")
	flags.StringSliceVarP(&imagesCommand.Filter, "filter", "f", []string{}, "Filter output based on conditions provided (default [])")
	flags.StringVar(&imagesCommand.Format, "format", "", "Change the output format to JSON or a Go template")
	flags.BoolVarP(&imagesCommand.Noheading, "noheading", "n", false, "Do not print column headings")
	// TODO Need to learn how to deal with second name being a string instead of a char.
	// This needs to be "no-trunc, notruncate"
	flags.BoolVar(&imagesCommand.NoTrunc, "no-trunc", false, "Do not truncate output")
	flags.BoolVarP(&imagesCommand.Quiet, "quiet", "q", false, "Display only image IDs")
	flags.StringVar(&imagesCommand.Sort, "sort", "created", "Sort by created, id, repository, size, or tag")

}

func imagesCmd(c *cliconfig.ImagesValues) error {
	var (
		filterFuncs []imagefilters.ResultFilter
		newImage    *adapter.ContainerImage
	)

	runtime, err := adapter.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "Could not get runtime")
	}
	defer runtime.Shutdown(false)
	if len(c.InputArgs) == 1 {
		newImage, err = runtime.NewImageFromLocal(c.InputArgs[0])
		if err != nil {
			return err
		}
	}

	if len(c.InputArgs) > 1 {
		return errors.New("'podman images' requires at most 1 argument")
	}

	ctx := getContext()

	if len(c.Filter) > 0 || newImage != nil {
		filterFuncs, err = CreateFilterFuncs(ctx, runtime, c.Filter, newImage)
		if err != nil {
			return err
		}
	}

	opts := imagesOptions{
		quiet:     c.Quiet,
		noHeading: c.Noheading,
		noTrunc:   c.NoTrunc,
		digests:   c.Digests,
		format:    c.Format,
		sort:      c.Sort,
		all:       c.All,
	}

	opts.outputformat = opts.setOutputFormat()
	images, err := runtime.GetImages()
	if err != nil {
		return errors.Wrapf(err, "unable to get images")
	}

	var filteredImages []*adapter.ContainerImage
	//filter the images
	if len(c.Filter) > 0 || newImage != nil {
		filteredImages = imagefilters.FilterImages(images, filterFuncs)
	} else {
		filteredImages = images
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
	format := "table {{.Repository}}\t{{.Tag}}\t"
	if i.noHeading {
		format = "{{.Repository}}\t{{.Tag}}\t"
	}
	if i.digests {
		format += "{{.Digest}}\t"
	}
	format += "{{.ID}}\t{{.Created}}\t{{.Size}}\t"
	return format
}

// imagesToGeneric creates an empty array of interfaces for output
func imagesToGeneric(templParams []imagesTemplateParams, JSONParams []imagesJSONParams) (genericParams []interface{}) {
	if len(templParams) > 0 {
		for _, v := range templParams {
			genericParams = append(genericParams, interface{}(v))
		}
		return
	}
	for _, v := range JSONParams {
		genericParams = append(genericParams, interface{}(v))
	}
	return
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
func getImagesTemplateOutput(ctx context.Context, images []*adapter.ContainerImage, opts imagesOptions) (imagesOutput imagesSorted) {
	for _, img := range images {
		// If all is false and the image doesn't have a name, check to see if the top layer of the image is a parent
		// to another image's top layer. If it is, then it is an intermediate image so don't print out if the --all flag
		// is not set.
		isParent, err := img.IsParent()
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

		// get all specified repo:tag pairs and print them separately
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
				params := imagesTemplateParams{
					Repository:  repo,
					Tag:         tag,
					ID:          imageID,
					Digest:      img.Digest(),
					CreatedTime: createdTime,
					Created:     units.HumanDuration(time.Since((createdTime))) + " ago",
					Size:        sizeStr,
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
	return
}

// getImagesJSONOutput returns the images information in its raw form
func getImagesJSONOutput(ctx context.Context, images []*adapter.ContainerImage) (imagesOutput []imagesJSONParams) {
	for _, img := range images {
		size, err := img.Size(ctx)
		if err != nil {
			size = nil
		}
		params := imagesJSONParams{
			ID:      img.ID(),
			Name:    img.Names(),
			Digest:  img.Digest(),
			Created: img.Created(),
			Size:    size,
		}
		imagesOutput = append(imagesOutput, params)
	}
	return
}

// generateImagesOutput generates the images based on the format provided

func generateImagesOutput(ctx context.Context, images []*adapter.ContainerImage, opts imagesOptions) error {
	templateMap := GenImageOutputMap()
	if len(images) == 0 {
		return nil
	}
	var out formats.Writer

	switch opts.format {
	case formats.JSONString:
		imagesOutput := getImagesJSONOutput(ctx, images)
		out = formats.JSONStructArray{Output: imagesToGeneric([]imagesTemplateParams{}, imagesOutput)}
	default:
		imagesOutput := getImagesTemplateOutput(ctx, images, opts)
		out = formats.StdoutTemplateArray{Output: imagesToGeneric(imagesOutput, []imagesJSONParams{}), Template: opts.outputformat, Fields: templateMap}
	}
	return formats.Writer(out).Out()
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
		values[key] = strings.ToUpper(splitCamelCase(value))
	}
	return values
}

// CreateFilterFuncs returns an array of filter functions based on the user inputs
// and is later used to filter images for output
func CreateFilterFuncs(ctx context.Context, r *adapter.LocalRuntime, filters []string, img *adapter.ContainerImage) ([]imagefilters.ResultFilter, error) {
	var filterFuncs []imagefilters.ResultFilter
	for _, filter := range filters {
		splitFilter := strings.Split(filter, "=")
		switch splitFilter[0] {
		case "before":
			before, err := r.NewImageFromLocal(splitFilter[1])
			if err != nil {
				return nil, errors.Wrapf(err, "unable to find image %s in local stores", splitFilter[1])
			}
			filterFuncs = append(filterFuncs, imagefilters.CreatedBeforeFilter(before.Created()))
		case "after":
			after, err := r.NewImageFromLocal(splitFilter[1])
			if err != nil {
				return nil, errors.Wrapf(err, "unable to find image %s in local stores", splitFilter[1])
			}
			filterFuncs = append(filterFuncs, imagefilters.CreatedAfterFilter(after.Created()))
		case "dangling":
			filterFuncs = append(filterFuncs, imagefilters.DanglingFilter())
		case "label":
			labelFilter := strings.Join(splitFilter[1:], "=")
			filterFuncs = append(filterFuncs, imagefilters.LabelFilter(ctx, labelFilter))
		default:
			return nil, errors.Errorf("invalid filter %s ", splitFilter[0])
		}
	}
	if img != nil {
		filterFuncs = append(filterFuncs, imagefilters.OutputImageFilter(img))
	}
	return filterFuncs, nil
}
