package main

import (
	js "encoding/json"
	"fmt"

	of "github.com/kubernetes-incubator/cri-o/cmd/kpod/formats"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	mountDescription = `
   kpod mount
   Lists all mounted containers mount points

   kpod mount CONTAINER-NAME-OR-ID
   Mounts the specified container and outputs the mountpoint
`

	mountFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "notruncate",
			Usage: "do not truncate output",
		},
		cli.StringFlag{
			Name:  "label",
			Usage: "SELinux label for the mount point",
		},
		cli.StringFlag{
			Name:  "format",
			Usage: "Change the output format to Go template",
		},
	}
	mountCommand = cli.Command{
		Name:        "mount",
		Usage:       "Mount a working container's root filesystem",
		Description: mountDescription,
		Action:      mountCmd,
		ArgsUsage:   "[CONTAINER-NAME-OR-ID]",
		Flags:       mountFlags,
	}
)

// MountOutputParams stores info about each layer
type jsonMountPoint struct {
	ID         string   `json:"id"`
	Names      []string `json:"names"`
	MountPoint string   `json:"mountpoint"`
}

func mountCmd(c *cli.Context) error {
	formats := map[string]bool{
		"":            true,
		of.JSONString: true,
	}

	args := c.Args()
	json := c.String("format") == of.JSONString
	if !formats[c.String("format")] {
		return errors.Errorf("%q is not a supported format", c.String("format"))
	}

	if len(args) > 1 {
		return errors.Errorf("too many arguments specified")
	}
	if err := validateFlags(c, mountFlags); err != nil {
		return err
	}
	config, err := getConfig(c)
	if err != nil {
		return errors.Wrapf(err, "Could not get config")
	}
	store, err := getStore(config)
	if err != nil {
		return errors.Wrapf(err, "error getting store")
	}
	if len(args) == 1 {
		if json {
			return errors.Wrapf(err, "json option can not be used with a container id")
		}
		mountPoint, err := store.Mount(args[0], c.String("label"))
		if err != nil {
			return errors.Wrapf(err, "error finding container %q", args[0])
		}
		fmt.Printf("%s\n", mountPoint)
	} else {
		jsonMountPoints := []jsonMountPoint{}
		containers, err2 := store.Containers()
		if err2 != nil {
			return errors.Wrapf(err2, "error reading list of all containers")
		}
		for _, container := range containers {
			layer, err := store.Layer(container.LayerID)
			if err != nil {
				return errors.Wrapf(err, "error finding layer %q for container %q", container.LayerID, container.ID)
			}
			if layer.MountPoint == "" {
				continue
			}
			if json {
				jsonMountPoints = append(jsonMountPoints, jsonMountPoint{ID: container.ID, Names: container.Names, MountPoint: layer.MountPoint})
				continue
			}

			if c.Bool("notruncate") {
				fmt.Printf("%-64s %s\n", container.ID, layer.MountPoint)
			} else {
				fmt.Printf("%-12.12s %s\n", container.ID, layer.MountPoint)
			}
		}
		if json {
			data, err := js.MarshalIndent(jsonMountPoints, "", "    ")
			if err != nil {
				return err
			}
			fmt.Printf("%s\n", data)
		}
	}
	return nil
}
