package main

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/containers/libpod/cmd/podman/formats"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	"github.com/docker/go-units"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
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
	imagesFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "Show all images (default hides intermediate images)",
		},
		cli.BoolFlag{
			Name:  "digests",
			Usage: "show digests",
		},
		cli.StringSliceFlag{
			Name:  "filter, f",
			Usage: "filter output based on conditions provided (default [])",
		},
		cli.StringFlag{
			Name:  "format",
			Usage: "Change the output format to JSON or a Go template",
		},
		cli.BoolFlag{
			Name:  "noheading, n",
			Usage: "do not print column headings",
		},
		cli.BoolFlag{
			Name:  "no-trunc, notruncate",
			Usage: "do not truncate output",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "display only image IDs",
		},
		cli.StringFlag{
			Name:  "sort",
			Usage: "Sort by created, id, repository, size, or tag",
			Value: "created",
		},
	}

	imagesDescription = "lists locally stored images."
	imagesCommand     = cli.Command{
		Name:                   "images",
		Usage:                  "List images in local storage",
		Description:            imagesDescription,
		Flags:                  imagesFlags,
		Action:                 imagesCmd,
		ArgsUsage:              "",
		UseShortOptionHandling: true,
		OnUsageError:           usageErrorHandler,
	}
	lsImagesCommand = cli.Command{
		Name:                   "ls",
		Usage:                  "list images in local storage",
		Description:            imagesDescription,
		Flags:                  imagesFlags,
		Action:                 imagesCmd,
		ArgsUsage:              "",
		UseShortOptionHandling: true,
		OnUsageError:           usageErrorHandler,
	}
)

func imagesCmd(c *cli.Context) error {
	var (
		filterFuncs []image.ResultFilter
		newImage    *image.Image
	)
	if err := validateFlags(c, imagesFlags); err != nil {
		return err
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "Could not get runtime")
	}
	defer runtime.Shutdown(false)
	if len(c.Args()) == 1 {
		newImage, err = runtime.ImageRuntime().NewFromLocal(c.Args().Get(0))
		if err != nil {
			return err
		}
	}

	if len(c.Args()) > 1 {
		return errors.New("'podman images' requires at most 1 argument")
	}

	ctx := getContext()

	if len(c.StringSlice("filter")) > 0 || newImage != nil {
		filterFuncs, err = CreateFilterFuncs(ctx, runtime, c, newImage)
		if err != nil {
			return err
		}
	}

	opts := imagesOptions{
		quiet:     c.Bool("quiet"),
		noHeading: c.Bool("noheading"),
		noTrunc:   c.Bool("no-trunc"),
		digests:   c.Bool("digests"),
		format:    c.String("format"),
		sort:      c.String("sort"),
		all:       c.Bool("all"),
	}

	opts.outputformat = opts.setOutputFormat()
	/*
		podman does not implement --all for images

		intermediate images are only generated during the build process.  they are
		children to the image once built. until buildah supports caching builds,
		it will not generate these intermediate images.
	*/
	images, err := runtime.ImageRuntime().GetImages()
	if err != nil {
		return errors.Wrapf(err, "unable to get images")
	}

	var filteredImages []*image.Image
	// filter the images
	if len(c.StringSlice("filter")) > 0 || newImage != nil {
		filteredImages = image.FilterImages(images, filterFuncs)
	} else {
		filteredImages = images
	}

	return generateImagesOutput(ctx, runtime, filteredImages, opts)
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
func getImagesTemplateOutput(ctx context.Context, runtime *libpod.Runtime, images []*image.Image, opts imagesOptions) (imagesOutput imagesSorted) {
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
		for repo, tags := range image.ReposToMap(img.Names()) {
			for _, tag := range tags {
				size, err := img.Size(ctx)
				if err != nil {
					size = nil
				}
				params := imagesTemplateParams{
					Repository:  repo,
					Tag:         tag,
					ID:          imageID,
					Digest:      img.Digest(),
					CreatedTime: createdTime,
					Created:     units.HumanDuration(time.Since((createdTime))) + " ago",
					Size:        units.HumanSizeWithPrecision(float64(*size), 3),
				}
				imagesOutput = append(imagesOutput, params)
			}
		}
	}

	// Sort images by created time
	sortImagesOutput(opts.sort, imagesOutput)
	return
}

// getImagesJSONOutput returns the images information in its raw form
func getImagesJSONOutput(ctx context.Context, runtime *libpod.Runtime, images []*image.Image) (imagesOutput []imagesJSONParams) {
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

func generateImagesOutput(ctx context.Context, runtime *libpod.Runtime, images []*image.Image, opts imagesOptions) error {
	if len(images) == 0 {
		return nil
	}
	var out formats.Writer

	switch opts.format {
	case formats.JSONString:
		imagesOutput := getImagesJSONOutput(ctx, runtime, images)
		out = formats.JSONStructArray{Output: imagesToGeneric([]imagesTemplateParams{}, imagesOutput)}
	default:
		imagesOutput := getImagesTemplateOutput(ctx, runtime, images, opts)
		out = formats.StdoutTemplateArray{Output: imagesToGeneric(imagesOutput, []imagesJSONParams{}), Template: opts.outputformat, Fields: imagesOutput[0].HeaderMap()}
	}
	return formats.Writer(out).Out()
}

// HeaderMap produces a generic map of "headers" based on a line
// of output
func (i *imagesTemplateParams) HeaderMap() map[string]string {
	v := reflect.Indirect(reflect.ValueOf(i))
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
func CreateFilterFuncs(ctx context.Context, r *libpod.Runtime, c *cli.Context, img *image.Image) ([]image.ResultFilter, error) {
	var filterFuncs []image.ResultFilter
	for _, filter := range c.StringSlice("filter") {
		splitFilter := strings.Split(filter, "=")
		switch splitFilter[0] {
		case "before":
			before, err := r.ImageRuntime().NewFromLocal(splitFilter[1])
			if err != nil {
				return nil, errors.Wrapf(err, "unable to find image % in local stores", splitFilter[1])
			}
			filterFuncs = append(filterFuncs, image.CreatedBeforeFilter(before.Created()))
		case "after":
			after, err := r.ImageRuntime().NewFromLocal(splitFilter[1])
			if err != nil {
				return nil, errors.Wrapf(err, "unable to find image % in local stores", splitFilter[1])
			}
			filterFuncs = append(filterFuncs, image.CreatedAfterFilter(after.Created()))
		case "dangling":
			filterFuncs = append(filterFuncs, image.DanglingFilter())
		case "label":
			labelFilter := strings.Join(splitFilter[1:], "=")
			filterFuncs = append(filterFuncs, image.LabelFilter(ctx, labelFilter))
		default:
			return nil, errors.Errorf("invalid filter %s ", splitFilter[0])
		}
	}
	if img != nil {
		filterFuncs = append(filterFuncs, image.OutputImageFilter(img))
	}
	return filterFuncs, nil
}
