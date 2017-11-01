package main

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/docker/go-units"
	"github.com/kubernetes-incubator/cri-o/cmd/kpod/formats"
	"github.com/kubernetes-incubator/cri-o/libpod"
	"github.com/kubernetes-incubator/cri-o/libpod/common"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

type imagesTemplateParams struct {
	ID        string
	Name      string
	Digest    digest.Digest
	CreatedAt string
	Size      string
}

type imagesJSONParams struct {
	ID        string        `json:"id"`
	Name      []string      `json:"names"`
	Digest    digest.Digest `json:"digest"`
	CreatedAt time.Time     `json:"created"`
	Size      int64         `json:"size"`
}

type imagesOptions struct {
	quiet     bool
	noHeading bool
	noTrunc   bool
	digests   bool
	format    string
}

var (
	imagesFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "display only image IDs",
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
			Name:  "digests",
			Usage: "show digests",
		},
		cli.StringFlag{
			Name:  "format",
			Usage: "Change the output format to JSON or a Go template",
		},
		cli.StringFlag{
			Name:  "filter, f",
			Usage: "filter output based on conditions provided (default [])",
		},
	}

	imagesDescription = "lists locally stored images."
	imagesCommand     = cli.Command{
		Name:        "images",
		Usage:       "list images in local storage",
		Description: imagesDescription,
		Flags:       imagesFlags,
		Action:      imagesCmd,
		ArgsUsage:   "",
	}
)

func imagesCmd(c *cli.Context) error {
	if err := validateFlags(c, imagesFlags); err != nil {
		return err
	}

	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "Could not get runtime")
	}
	defer runtime.Shutdown(false)

	var format string
	if c.IsSet("format") {
		format = c.String("format")
	} else {
		format = genImagesFormat(c.Bool("quiet"), c.Bool("noheading"), c.Bool("digests"))
	}

	opts := imagesOptions{
		quiet:     c.Bool("quiet"),
		noHeading: c.Bool("noheading"),
		noTrunc:   c.Bool("no-trunc"),
		digests:   c.Bool("digests"),
		format:    format,
	}

	var imageInput string
	if len(c.Args()) == 1 {
		imageInput = c.Args().Get(0)
	}
	if len(c.Args()) > 1 {
		return errors.New("'kpod images' requires at most 1 argument")
	}

	params, err := runtime.ParseImageFilter(imageInput, c.String("filter"))
	if err != nil {
		return errors.Wrapf(err, "error parsing filter")
	}

	// generate the different filters
	labelFilter := generateImagesFilter(params, "label")
	beforeImageFilter := generateImagesFilter(params, "before-image")
	sinceImageFilter := generateImagesFilter(params, "since-image")
	danglingFilter := generateImagesFilter(params, "dangling")
	referenceFilter := generateImagesFilter(params, "reference")
	imageInputFilter := generateImagesFilter(params, "image-input")

	images, err := runtime.GetImages(params, labelFilter, beforeImageFilter, sinceImageFilter, danglingFilter, referenceFilter, imageInputFilter)
	if err != nil {
		return errors.Wrapf(err, "could not get list of images matching filter")
	}

	return generateImagesOutput(runtime, images, opts)
}

func genImagesFormat(quiet, noHeading, digests bool) (format string) {
	if quiet {
		return formats.IDString
	}
	format = "table {{.ID}}\t{{.Name}}\t"
	if noHeading {
		format = "{{.ID}}\t{{.Name}}\t"
	}
	if digests {
		format += "{{.Digest}}\t"
	}
	format += "{{.CreatedAt}}\t{{.Size}}\t"
	return
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

// generate the header based on the template provided
func (i *imagesTemplateParams) headerMap() map[string]string {
	v := reflect.Indirect(reflect.ValueOf(i))
	values := make(map[string]string)

	for i := 0; i < v.NumField(); i++ {
		key := v.Type().Field(i).Name
		value := key
		if value == "ID" || value == "Name" {
			value = "Image" + value
		}
		values[key] = strings.ToUpper(splitCamelCase(value))
	}
	return values
}

// getImagesTemplateOutput returns the images information to be printed in human readable format
func getImagesTemplateOutput(runtime *libpod.Runtime, images []*storage.Image, opts imagesOptions) (imagesOutput []imagesTemplateParams) {
	var (
		lastID string
	)
	for _, img := range images {
		if opts.quiet && lastID == img.ID {
			continue // quiet should not show the same ID multiple times
		}
		createdTime := img.Created

		imageID := img.ID
		if !opts.noTrunc {
			imageID = imageID[:idTruncLength]
		}

		imageName := "<none>"
		if len(img.Names) > 0 {
			imageName = img.Names[0]
		}

		info, imageDigest, size, _ := runtime.InfoAndDigestAndSize(*img)
		if info != nil {
			createdTime = info.Created
		}

		params := imagesTemplateParams{
			ID:        imageID,
			Name:      imageName,
			Digest:    imageDigest,
			CreatedAt: units.HumanDuration(time.Since((createdTime))) + " ago",
			Size:      units.HumanSize(float64(size)),
		}
		imagesOutput = append(imagesOutput, params)
	}
	return
}

// getImagesJSONOutput returns the images information in its raw form
func getImagesJSONOutput(runtime *libpod.Runtime, images []*storage.Image) (imagesOutput []imagesJSONParams) {
	for _, img := range images {
		createdTime := img.Created

		info, imageDigest, size, _ := runtime.InfoAndDigestAndSize(*img)
		if info != nil {
			createdTime = info.Created
		}

		params := imagesJSONParams{
			ID:        img.ID,
			Name:      img.Names,
			Digest:    imageDigest,
			CreatedAt: createdTime,
			Size:      size,
		}
		imagesOutput = append(imagesOutput, params)
	}
	return
}

// generateImagesOutput generates the images based on the format provided
func generateImagesOutput(runtime *libpod.Runtime, images []*storage.Image, opts imagesOptions) error {
	if len(images) == 0 {
		return nil
	}

	var out formats.Writer

	switch opts.format {
	case formats.JSONString:
		imagesOutput := getImagesJSONOutput(runtime, images)
		out = formats.JSONStructArray{Output: imagesToGeneric([]imagesTemplateParams{}, imagesOutput)}
	default:
		imagesOutput := getImagesTemplateOutput(runtime, images, opts)
		out = formats.StdoutTemplateArray{Output: imagesToGeneric(imagesOutput, []imagesJSONParams{}), Template: opts.format, Fields: imagesOutput[0].headerMap()}

	}

	return formats.Writer(out).Out()
}

// generateImagesFilter returns an ImageFilter based on filterType
// to add more filters, define a new case and write what the ImageFilter function should do
func generateImagesFilter(params *libpod.ImageFilterParams, filterType string) libpod.ImageFilter {
	switch filterType {
	case "label":
		return func(image *storage.Image, info *types.ImageInspectInfo) bool {
			if params == nil || params.Label == "" {
				return true
			}

			pair := strings.SplitN(params.Label, "=", 2)
			if val, ok := info.Labels[pair[0]]; ok {
				if len(pair) == 2 && val == pair[1] {
					return true
				}
				if len(pair) == 1 {
					return true
				}
			}
			return false
		}
	case "before-image":
		return func(image *storage.Image, info *types.ImageInspectInfo) bool {
			if params == nil || params.BeforeImage.IsZero() {
				return true
			}
			return info.Created.Before(params.BeforeImage)
		}
	case "since-image":
		return func(image *storage.Image, info *types.ImageInspectInfo) bool {
			if params == nil || params.SinceImage.IsZero() {
				return true
			}
			return info.Created.After(params.SinceImage)
		}
	case "dangling":
		return func(image *storage.Image, info *types.ImageInspectInfo) bool {
			if params == nil || params.Dangling == "" {
				return true
			}
			if common.IsFalse(params.Dangling) && params.ImageName != "<none>" {
				return true
			}
			if common.IsTrue(params.Dangling) && params.ImageName == "<none>" {
				return true
			}
			return false
		}
	case "reference":
		return func(image *storage.Image, info *types.ImageInspectInfo) bool {
			if params == nil || params.ReferencePattern == "" {
				return true
			}
			return libpod.MatchesReference(params.ImageName, params.ReferencePattern)
		}
	case "image-input":
		return func(image *storage.Image, info *types.ImageInspectInfo) bool {
			if params == nil || params.ImageInput == "" {
				return true
			}
			return libpod.MatchesReference(params.ImageName, params.ImageInput)
		}
	default:
		fmt.Println("invalid filter type", filterType)
		return nil
	}
}
