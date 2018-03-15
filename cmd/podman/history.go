package main

import (
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/containers/image/types"
	units "github.com/docker/go-units"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/cmd/podman/formats"
	"github.com/urfave/cli"
)

const createdByTruncLength = 45

// historyTemplateParams stores info about each layer
type historyTemplateParams struct {
	ID        string
	Created   string
	CreatedBy string
	Size      string
	Comment   string
}

// historyJSONParams is only used when the JSON format is specified,
// and is better for data processing from JSON.
// historyJSONParams will be populated by data from v1.History and types.BlobInfo,
// the members of the struct are the sama data types as their sources.
type historyJSONParams struct {
	ID        string     `json:"id"`
	Created   *time.Time `json:"created"`
	CreatedBy string     `json:"createdBy"`
	Size      int64      `json:"size"`
	Comment   string     `json:"comment"`
}

// historyOptions stores cli flag values
type historyOptions struct {
	human   bool
	noTrunc bool
	quiet   bool
	format  string
}

var (
	historyFlags = []cli.Flag{
		cli.BoolTFlag{
			Name:  "human, H",
			Usage: "Display sizes and dates in human readable format",
		},
		cli.BoolFlag{
			Name:  "no-trunc, notruncate",
			Usage: "Do not truncate the output",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Display the numeric IDs only",
		},
		cli.StringFlag{
			Name:  "format",
			Usage: "Change the output to JSON or a Go template",
		},
	}

	historyDescription = "Displays the history of an image. The information can be printed out in an easy to read, " +
		"or user specified format, and can be truncated."
	historyCommand = cli.Command{
		Name:                   "history",
		Usage:                  "Show history of a specified image",
		Description:            historyDescription,
		Flags:                  historyFlags,
		Action:                 historyCmd,
		ArgsUsage:              "",
		UseShortOptionHandling: true,
	}
)

func historyCmd(c *cli.Context) error {
	if err := validateFlags(c, historyFlags); err != nil {
		return err
	}

	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	format := genHistoryFormat(c.String("format"), c.Bool("quiet"))

	args := c.Args()
	if len(args) == 0 {
		return errors.Errorf("an image name must be specified")
	}
	if len(args) > 1 {
		return errors.Errorf("podman history takes at most 1 argument")
	}

	image, err := runtime.ImageRuntime().NewFromLocal(args[0])
	if err != nil {
		return err
	}
	opts := historyOptions{
		human:   c.BoolT("human"),
		noTrunc: c.Bool("no-trunc"),
		quiet:   c.Bool("quiet"),
		format:  format,
	}

	history, layers, err := image.History()
	if err != nil {
		return errors.Wrapf(err, "error getting history of image %q", image.InputName)
	}

	return generateHistoryOutput(history, layers, image.ID(), opts)
}

func genHistoryFormat(format string, quiet bool) string {
	if format != "" {
		// "\t" from the command line is not being recognized as a tab
		// replacing the string "\t" to a tab character if the user passes in "\t"
		return strings.Replace(format, `\t`, "\t", -1)
	}
	if quiet {
		return formats.IDString
	}
	return "table {{.ID}}\t{{.Created}}\t{{.CreatedBy}}\t{{.Size}}\t{{.Comment}}\t"
}

// historyToGeneric makes an empty array of interfaces for output
func historyToGeneric(templParams []historyTemplateParams, JSONParams []historyJSONParams) (genericParams []interface{}) {
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
func (h *historyTemplateParams) headerMap() map[string]string {
	v := reflect.Indirect(reflect.ValueOf(h))
	values := make(map[string]string)
	for h := 0; h < v.NumField(); h++ {
		key := v.Type().Field(h).Name
		value := key
		values[key] = strings.ToUpper(splitCamelCase(value))
	}
	return values
}

// getHistorytemplateOutput gets the modified history information to be printed in human readable format
func getHistoryTemplateOutput(history []v1.History, layers []types.BlobInfo, imageID string, opts historyOptions) (historyOutput []historyTemplateParams) {
	var (
		outputSize  string
		createdTime string
		createdBy   string
		count       = 1
	)
	for i := len(history) - 1; i >= 0; i-- {
		if i != len(history)-1 {
			imageID = "<missing>"
		}
		if !opts.noTrunc && i == len(history)-1 {
			imageID = shortID(imageID)
		}

		var size int64
		if !history[i].EmptyLayer {
			size = layers[len(layers)-count].Size
			count++
		}

		if opts.human {
			createdTime = units.HumanDuration(time.Since((*history[i].Created))) + " ago"
			outputSize = units.HumanSize(float64(size))
		} else {
			createdTime = (history[i].Created).Format(time.RFC3339)
			outputSize = strconv.FormatInt(size, 10)
		}

		createdBy = strings.Join(strings.Fields(history[i].CreatedBy), " ")
		if !opts.noTrunc && len(createdBy) > createdByTruncLength {
			createdBy = createdBy[:createdByTruncLength-3] + "..."
		}

		params := historyTemplateParams{
			ID:        imageID,
			Created:   createdTime,
			CreatedBy: createdBy,
			Size:      outputSize,
			Comment:   history[i].Comment,
		}
		historyOutput = append(historyOutput, params)
	}
	return
}

// getHistoryJSONOutput returns the history information in its raw form
func getHistoryJSONOutput(history []v1.History, layers []types.BlobInfo, imageID string) (historyOutput []historyJSONParams) {
	count := 1
	for i := len(history) - 1; i >= 0; i-- {
		var size int64
		if !history[i].EmptyLayer {
			size = layers[len(layers)-count].Size
			count++
		}

		params := historyJSONParams{
			ID:        imageID,
			Created:   history[i].Created,
			CreatedBy: history[i].CreatedBy,
			Size:      size,
			Comment:   history[i].Comment,
		}
		historyOutput = append(historyOutput, params)
	}
	return
}

// generateHistoryOutput generates the history based on the format given
func generateHistoryOutput(history []v1.History, layers []types.BlobInfo, imageID string, opts historyOptions) error {
	if len(history) == 0 {
		return nil
	}

	var out formats.Writer

	switch opts.format {
	case formats.JSONString:
		historyOutput := getHistoryJSONOutput(history, layers, imageID)
		out = formats.JSONStructArray{Output: historyToGeneric([]historyTemplateParams{}, historyOutput)}
	default:
		historyOutput := getHistoryTemplateOutput(history, layers, imageID, opts)
		out = formats.StdoutTemplateArray{Output: historyToGeneric(historyOutput, []historyJSONParams{}), Template: opts.format, Fields: historyOutput[0].headerMap()}
	}

	return formats.Writer(out).Out()
}
