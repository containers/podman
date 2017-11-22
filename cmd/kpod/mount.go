package main

import (
	js "encoding/json"
	"fmt"

	"github.com/pkg/errors"
	of "github.com/projectatomic/libpod/cmd/kpod/formats"
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

// jsonMountPoint stores info about each container
type jsonMountPoint struct {
	ID         string   `json:"id"`
	Names      []string `json:"names"`
	MountPoint string   `json:"mountpoint"`
}

func mountCmd(c *cli.Context) error {
	if err := validateFlags(c, mountFlags); err != nil {
		return err
	}

	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

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

	if len(args) == 1 {
		if json {
			return errors.Wrapf(err, "json option can not be used with a container id")
		}
		ctr, err := runtime.LookupContainer(args[0])
		if err != nil {
			return errors.Wrapf(err, "error looking up container %q", args[0])
		}
		mountPoint, err := ctr.Mount(c.String("label"))
		if err != nil {
			return errors.Wrapf(err, "error mounting container %q", ctr.ID())
		}
		fmt.Printf("%s\n", mountPoint)
	} else {
		jsonMountPoints := []jsonMountPoint{}
		containers, err2 := runtime.GetContainers()
		if err2 != nil {
			return errors.Wrapf(err2, "error reading list of all containers")
		}
		for _, container := range containers {
			mountPoint, err := container.MountPoint()
			if err != nil {
				return errors.Wrapf(err, "error getting mountpoint for %q", container.ID())
			}
			if mountPoint == "" {
				continue
			}
			if json {
				jsonMountPoints = append(jsonMountPoints, jsonMountPoint{ID: container.ID(), Names: []string{container.Name()}, MountPoint: mountPoint})
				continue
			}

			if c.Bool("notruncate") {
				fmt.Printf("%-64s %s\n", container.ID(), mountPoint)
			} else {
				fmt.Printf("%-12.12s %s\n", container.ID(), mountPoint)
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
