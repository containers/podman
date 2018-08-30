package main

import (
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/containers/libpod/cmd/podman/formats"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod/image"
	units "github.com/docker/go-units"
	"github.com/pkg/errors"
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
		OnUsageError:           usageErrorHandler,
	}
)

func historyCmd(c *cli.Context) error {
	if err := validateFlags(c, historyFlags); err != nil {
		return err
	}

	runtime, err := libpodruntime.GetRuntime(c)
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

	history, err := image.History(getContext())
	if err != nil {
		return errors.Wrapf(err, "error getting history of image %q", image.InputName)
	}

	return generateHistoryOutput(history, opts)
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
func historyToGeneric(templParams []historyTemplateParams, JSONParams []*image.History) (genericParams []interface{}) {
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
func getHistoryTemplateOutput(history []*image.History, opts historyOptions) (historyOutput []historyTemplateParams) {
	var (
		outputSize  string
		createdTime string
		createdBy   string
	)
	for _, hist := range history {
		imageID := hist.ID
		if !opts.noTrunc && imageID != "<missing>" {
			imageID = shortID(imageID)
		}

		if opts.human {
			createdTime = units.HumanDuration(time.Since((*hist.Created))) + " ago"
			outputSize = units.HumanSize(float64(hist.Size))
		} else {
			createdTime = (hist.Created).Format(time.RFC3339)
			outputSize = strconv.FormatInt(hist.Size, 10)
		}

		createdBy = strings.Join(strings.Fields(hist.CreatedBy), " ")
		if !opts.noTrunc && len(createdBy) > createdByTruncLength {
			createdBy = createdBy[:createdByTruncLength-3] + "..."
		}

		params := historyTemplateParams{
			ID:        imageID,
			Created:   createdTime,
			CreatedBy: createdBy,
			Size:      outputSize,
			Comment:   hist.Comment,
		}
		historyOutput = append(historyOutput, params)
	}
	return
}

// generateHistoryOutput generates the history based on the format given
func generateHistoryOutput(history []*image.History, opts historyOptions) error {
	if len(history) == 0 {
		return nil
	}

	var out formats.Writer

	switch opts.format {
	case formats.JSONString:
		out = formats.JSONStructArray{Output: historyToGeneric([]historyTemplateParams{}, history)}
	default:
		historyOutput := getHistoryTemplateOutput(history, opts)
		out = formats.StdoutTemplateArray{Output: historyToGeneric(historyOutput, []*image.History{}), Template: opts.format, Fields: historyOutput[0].headerMap()}
	}

	return formats.Writer(out).Out()
}
