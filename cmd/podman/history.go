package main

import (
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/formats"
	"github.com/containers/libpod/libpod/adapter"
	"github.com/containers/libpod/libpod/image"
	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
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
	historyCommand cliconfig.HistoryValues

	historyDescription = "Displays the history of an image. The information can be printed out in an easy to read, " +
		"or user specified format, and can be truncated."
	_historyCommand = &cobra.Command{
		Use:   "history",
		Short: "Show history of a specified image",
		Long:  historyDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			historyCommand.InputArgs = args
			historyCommand.GlobalFlags = MainGlobalOpts
			return historyCmd(&historyCommand)
		},
	}
)

func init() {
	historyCommand.Command = _historyCommand
	flags := historyCommand.Flags()
	flags.StringVar(&historyCommand.Format, "format", "", "Change the output to JSON or a Go template")
	flags.BoolVarP(&historyCommand.Human, "human", "H", true, "Display sizes and dates in human readable format")
	// notrucate needs to be added
	flags.BoolVar(&historyCommand.NoTrunc, "no-trunc", false, "Do not truncate the output")
	flags.BoolVarP(&historyCommand.Quiet, "quiet", "q", false, "Display the numeric IDs only")

}

func historyCmd(c *cliconfig.HistoryValues) error {
	runtime, err := adapter.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	format := genHistoryFormat(c.Format, c.Quiet)

	args := c.InputArgs
	if len(args) == 0 {
		return errors.Errorf("an image name must be specified")
	}
	if len(args) > 1 {
		return errors.Errorf("podman history takes at most 1 argument")
	}

	image, err := runtime.NewImageFromLocal(args[0])
	if err != nil {
		return err
	}
	opts := historyOptions{
		human:   c.Human,
		noTrunc: c.NoTrunc,
		quiet:   c.Quiet,
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
