package main

import (
	"github.com/kubernetes-incubator/cri-o/cmd/kpod/formats"
	"github.com/kubernetes-incubator/cri-o/libkpod"
	"github.com/kubernetes-incubator/cri-o/libpod/images"
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
	}
	inspectDescription = "This displays the low-level information on containers and images identified by name or ID. By default, this will render all results in a JSON array. If the container and image have the same name, this will return container JSON for unspecified type."
	inspectCommand     = cli.Command{
		Name:        "inspect",
		Usage:       "Displays the configuration of a container or image",
		Description: inspectDescription,
		Flags:       inspectFlags,
		Action:      inspectCmd,
		ArgsUsage:   "CONTAINER-OR-IMAGE",
	}
)

func inspectCmd(c *cli.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return errors.Errorf("container or image name must be specified: kpod inspect [options [...]] name")
	}
	if len(args) > 1 {
		return errors.Errorf("too many arguments specified")
	}
	if err := validateFlags(c, inspectFlags); err != nil {
		return err
	}

	itemType := c.String("type")
	size := c.Bool("size")

	switch itemType {
	case inspectTypeContainer:
	case inspectTypeImage:
	case inspectAll:
	default:
		return errors.Errorf("the only recognized types are %q, %q, and %q", inspectTypeContainer, inspectTypeImage, inspectAll)
	}

	name := args[0]

	config, err := getConfig(c)
	if err != nil {
		return errors.Wrapf(err, "Could not get config")
	}
	server, err := libkpod.New(config)
	if err != nil {
		return errors.Wrapf(err, "could not get container server")
	}
	defer server.Shutdown()
	if err = server.Update(); err != nil {
		return errors.Wrapf(err, "could not update list of containers")
	}

	outputFormat := c.String("format")
	var data interface{}
	switch itemType {
	case inspectTypeContainer:
		data, err = server.GetContainerData(name, size)
		if err != nil {
			return errors.Wrapf(err, "error parsing container data")
		}
	case inspectTypeImage:
		data, err = images.GetData(server.Store(), name)
		if err != nil {
			return errors.Wrapf(err, "error parsing image data")
		}
	case inspectAll:
		ctrData, err := server.GetContainerData(name, size)
		if err != nil {
			imgData, err := images.GetData(server.Store(), name)
			if err != nil {
				return errors.Wrapf(err, "error parsing container or image data")
			}
			data = imgData

		} else {
			data = ctrData
		}
	}

	var out formats.Writer
	if outputFormat != "" && outputFormat != formats.JSONString {
		//template
		out = formats.StdoutTemplate{Output: data, Template: outputFormat}
	} else {
		// default is json output
		out = formats.JSONStruct{Output: data}
	}

	formats.Writer(out).Out()
	return nil
}
