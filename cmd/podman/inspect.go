package main

import (
	"context"
	"strings"

	"github.com/containers/libpod/cmd/podman/formats"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

const (
	inspectTypeContainer = "container"
	inspectTypeImage     = "image"
	inspectAll           = "all"
)

var (
	inspectFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "type, t",
			Value: inspectAll,
			Usage: "Return JSON for specified type, (e.g image, container or task)",
		},
		cli.StringFlag{
			Name:  "format, f",
			Usage: "Change the output format to a Go template",
		},
		cli.BoolFlag{
			Name:  "size",
			Usage: "Display total file size if the type is container",
		},
		LatestFlag,
	}
	inspectDescription = "This displays the low-level information on containers and images identified by name or ID. By default, this will render all results in a JSON array. If the container and image have the same name, this will return container JSON for unspecified type."
	inspectCommand     = cli.Command{
		Name:         "inspect",
		Usage:        "Displays the configuration of a container or image",
		Description:  inspectDescription,
		Flags:        sortFlags(inspectFlags),
		Action:       inspectCmd,
		ArgsUsage:    "CONTAINER-OR-IMAGE [CONTAINER-OR-IMAGE]...",
		OnUsageError: usageErrorHandler,
	}
)

func inspectCmd(c *cli.Context) error {
	args := c.Args()
	inspectType := c.String("type")
	latestContainer := c.Bool("latest")
	if len(args) == 0 && !latestContainer {
		return errors.Errorf("container or image name must be specified: podman inspect [options [...]] name")
	}

	if len(args) > 0 && latestContainer {
		return errors.Errorf("you cannot provide additional arguments with --latest")
	}
	if err := validateFlags(c, inspectFlags); err != nil {
		return err
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	if !util.StringInSlice(inspectType, []string{inspectTypeContainer, inspectTypeImage, inspectAll}) {
		return errors.Errorf("the only recognized types are %q, %q, and %q", inspectTypeContainer, inspectTypeImage, inspectAll)
	}

	outputFormat := c.String("format")
	if strings.Contains(outputFormat, "{{.Id}}") {
		outputFormat = strings.Replace(outputFormat, "{{.Id}}", formats.IDString, -1)
	}
	if latestContainer {
		lc, err := runtime.GetLatestContainer()
		if err != nil {
			return err
		}
		args = append(args, lc.ID())
		inspectType = inspectTypeContainer
	}

	inspectedObjects, iterateErr := iterateInput(getContext(), c, args, runtime, inspectType)

	var out formats.Writer
	if outputFormat != "" && outputFormat != formats.JSONString {
		//template
		out = formats.StdoutTemplateArray{Output: inspectedObjects, Template: outputFormat}
	} else {
		// default is json output
		out = formats.JSONStructArray{Output: inspectedObjects}
	}

	formats.Writer(out).Out()
	return iterateErr
}

// func iterateInput iterates the images|containers the user has requested and returns the inspect data and error
func iterateInput(ctx context.Context, c *cli.Context, args []string, runtime *libpod.Runtime, inspectType string) ([]interface{}, error) {
	var (
		data           interface{}
		inspectedItems []interface{}
		inspectError   error
	)

	for _, input := range args {
		switch inspectType {
		case inspectTypeContainer:
			ctr, err := runtime.LookupContainer(input)
			if err != nil {
				inspectError = errors.Wrapf(err, "error looking up container %q", input)
				break
			}
			libpodInspectData, err := ctr.Inspect(c.Bool("size"))
			if err != nil {
				inspectError = errors.Wrapf(err, "error getting libpod container inspect data %s", ctr.ID())
				break
			}
			data, err = shared.GetCtrInspectInfo(ctr, libpodInspectData)
			if err != nil {
				inspectError = errors.Wrapf(err, "error parsing container data %q", ctr.ID())
				break
			}
		case inspectTypeImage:
			image, err := runtime.ImageRuntime().NewFromLocal(input)
			if err != nil {
				inspectError = errors.Wrapf(err, "error getting image %q", input)
				break
			}
			data, err = image.Inspect(ctx)
			if err != nil {
				inspectError = errors.Wrapf(err, "error parsing image data %q", image.ID())
				break
			}
		case inspectAll:
			ctr, err := runtime.LookupContainer(input)
			if err != nil {
				image, err := runtime.ImageRuntime().NewFromLocal(input)
				if err != nil {
					inspectError = errors.Wrapf(err, "error getting image %q", input)
					break
				}
				data, err = image.Inspect(ctx)
				if err != nil {
					inspectError = errors.Wrapf(err, "error parsing image data %q", image.ID())
					break
				}
			} else {
				libpodInspectData, err := ctr.Inspect(c.Bool("size"))
				if err != nil {
					inspectError = errors.Wrapf(err, "error getting libpod container inspect data %s", ctr.ID())
					break
				}
				data, err = shared.GetCtrInspectInfo(ctr, libpodInspectData)
				if err != nil {
					inspectError = errors.Wrapf(err, "error parsing container data %s", ctr.ID())
					break
				}
			}
		}
		if inspectError == nil {
			inspectedItems = append(inspectedItems, data)
		}
	}
	return inspectedItems, inspectError
}
