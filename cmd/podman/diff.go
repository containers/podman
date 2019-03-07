package main

import (
	"fmt"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/formats"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/storage/pkg/archive"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type diffJSONOutput struct {
	Changed []string `json:"changed,omitempty"`
	Added   []string `json:"added,omitempty"`
	Deleted []string `json:"deleted,omitempty"`
}

type diffOutputParams struct {
	Change archive.ChangeType
	Path   string
}

type stdoutStruct struct {
	output []diffOutputParams
}

func (so stdoutStruct) Out() error {
	for _, d := range so.output {
		fmt.Printf("%s %s\n", d.Change, d.Path)
	}
	return nil
}

var (
	diffCommand     cliconfig.DiffValues
	diffDescription = fmt.Sprint(`Displays changes on a container or image's filesystem.  The container or image will be compared to its parent layer.`)

	_diffCommand = &cobra.Command{
		Use:   "diff [flags] CONTAINER | IMAGE",
		Short: "Inspect changes on container's file systems",
		Long:  diffDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			diffCommand.InputArgs = args
			diffCommand.GlobalFlags = MainGlobalOpts
			return diffCmd(&diffCommand)
		},
		Example: `podman diff imageID
  podman diff ctrID
  podman diff --format json redis:alpine`,
	}
)

func init() {
	diffCommand.Command = _diffCommand
	diffCommand.SetHelpTemplate(HelpTemplate())
	diffCommand.SetUsageTemplate(UsageTemplate())
	flags := diffCommand.Flags()

	flags.BoolVar(&diffCommand.Archive, "archive", true, "Save the diff as a tar archive")
	flags.StringVar(&diffCommand.Format, "format", "", "Change the output format")

	flags.MarkHidden("archive")

}

func formatJSON(output []diffOutputParams) (diffJSONOutput, error) {
	jsonStruct := diffJSONOutput{}
	for _, output := range output {
		switch output.Change {
		case archive.ChangeModify:
			jsonStruct.Changed = append(jsonStruct.Changed, output.Path)
		case archive.ChangeAdd:
			jsonStruct.Added = append(jsonStruct.Added, output.Path)
		case archive.ChangeDelete:
			jsonStruct.Deleted = append(jsonStruct.Deleted, output.Path)
		default:
			return jsonStruct, errors.Errorf("output kind %q not recognized", output.Change.String())
		}
	}
	return jsonStruct, nil
}

func diffCmd(c *cliconfig.DiffValues) error {
	if len(c.InputArgs) != 1 {
		return errors.Errorf("container, image, or layer name must be specified: podman diff [options [...]] ID-NAME")
	}

	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	to := c.InputArgs[0]
	changes, err := runtime.GetDiff("", to)
	if err != nil {
		return errors.Wrapf(err, "could not get changes for %q", to)
	}

	diffOutput := []diffOutputParams{}
	outputFormat := c.Format

	for _, change := range changes {

		params := diffOutputParams{
			Change: change.Kind,
			Path:   change.Path,
		}
		diffOutput = append(diffOutput, params)
	}

	var out formats.Writer

	if outputFormat != "" {
		switch outputFormat {
		case formats.JSONString:
			data, err := formatJSON(diffOutput)
			if err != nil {
				return err
			}
			out = formats.JSONStruct{Output: data}
		default:
			return errors.New("only valid format for diff is 'json'")
		}
	} else {
		out = stdoutStruct{output: diffOutput}
	}
	formats.Writer(out).Out()

	return nil
}
