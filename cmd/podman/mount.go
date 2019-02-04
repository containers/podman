package main

import (
	js "encoding/json"
	"fmt"
	"os"

	of "github.com/containers/libpod/cmd/podman/formats"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	mountDescription = `
   podman mount
   Lists all mounted containers mount points

   podman mount CONTAINER-NAME-OR-ID
   Mounts the specified container and outputs the mountpoint
`

	mountFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "Mount all containers",
		},
		cli.StringFlag{
			Name:  "format",
			Usage: "Change the output format to Go template",
		},
		cli.BoolFlag{
			Name:  "notruncate",
			Usage: "Do not truncate output",
		},
		LatestFlag,
	}
	mountCommand = cli.Command{
		Name:         "mount",
		Usage:        "Mount a working container's root filesystem",
		Description:  mountDescription,
		Action:       mountCmd,
		ArgsUsage:    "[CONTAINER-NAME-OR-ID [...]]",
		Flags:        sortFlags(mountFlags),
		OnUsageError: usageErrorHandler,
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
	if os.Geteuid() != 0 {
		rootless.SetSkipStorageSetup(true)
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	if os.Geteuid() != 0 {
		if driver := runtime.GetConfig().StorageConfig.GraphDriverName; driver != "vfs" {
			// Do not allow to mount a graphdriver that is not vfs if we are creating the userns as part
			// of the mount command.
			return fmt.Errorf("cannot mount using driver %s in rootless mode", driver)
		}

		became, ret, err := rootless.BecomeRootInUserNS()
		if err != nil {
			return err
		}
		if became {
			os.Exit(ret)
		}
	}

	if c.Bool("all") && c.Bool("latest") {
		return errors.Errorf("--all and --latest cannot be used together")
	}

	mountContainers, err := getAllOrLatestContainers(c, runtime, -1, "all")
	if err != nil {
		if len(mountContainers) == 0 {
			return err
		}
		fmt.Println(err.Error())
	}

	formats := map[string]bool{
		"":            true,
		of.JSONString: true,
	}

	json := c.String("format") == of.JSONString
	if !formats[c.String("format")] {
		return errors.Errorf("%q is not a supported format", c.String("format"))
	}

	var lastError error
	if len(mountContainers) > 0 {
		for _, ctr := range mountContainers {
			if json {
				if lastError != nil {
					logrus.Error(lastError)
				}
				lastError = errors.Wrapf(err, "json option cannot be used with a container id")
				continue
			}
			mountPoint, err := ctr.Mount()
			if err != nil {
				if lastError != nil {
					logrus.Error(lastError)
				}
				lastError = errors.Wrapf(err, "error mounting container %q", ctr.ID())
				continue
			}
			fmt.Printf("%s\n", mountPoint)
		}
		return lastError
	} else {
		jsonMountPoints := []jsonMountPoint{}
		containers, err2 := runtime.GetContainers()
		if err2 != nil {
			return errors.Wrapf(err2, "error reading list of all containers")
		}
		for _, container := range containers {
			mounted, mountPoint, err := container.Mounted()
			if err != nil {
				return errors.Wrapf(err, "error getting mountpoint for %q", container.ID())
			}

			if !mounted {
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
