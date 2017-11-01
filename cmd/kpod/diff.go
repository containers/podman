package main

import (
	"fmt"

	"github.com/containers/storage/pkg/archive"
	"github.com/kubernetes-incubator/cri-o/cmd/kpod/formats"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
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
	diffFlags = []cli.Flag{
		cli.BoolFlag{
			Name:   "archive",
			Usage:  "Save the diff as a tar archive",
			Hidden: true,
		},
		cli.StringFlag{
			Name:  "format",
			Usage: "Change the output format.",
		},
	}
	diffDescription = fmt.Sprint(`Displays changes on a container or image's filesystem.  The
	container or image will be compared to its parent layer`)

	diffCommand = cli.Command{
		Name:        "diff",
		Usage:       "Inspect changes on container's file systems",
		Description: diffDescription,
		Flags:       diffFlags,
		Action:      diffCmd,
		ArgsUsage:   "ID-NAME",
	}
)

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

func diffCmd(c *cli.Context) error {
	if err := validateFlags(c, diffFlags); err != nil {
		return err
	}

	if len(c.Args()) != 1 {
		return errors.Errorf("container, image, or layer name must be specified: kpod diff [options [...]] ID-NAME")
	}

	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	to := c.Args().Get(0)
	changes, err := runtime.GetDiff("", to)
	if err != nil {
		return errors.Wrapf(err, "could not get changes for %q", to)
	}

	diffOutput := []diffOutputParams{}
	outputFormat := c.String("format")

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
