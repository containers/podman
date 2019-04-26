package main

import (
	js "encoding/json"
	"fmt"
	"os"

	of "github.com/containers/buildah/pkg/formats"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	mountCommand cliconfig.MountValues

	mountDescription = `podman mount
    Lists all mounted containers mount points if no container is specified

  podman mount CONTAINER-NAME-OR-ID
    Mounts the specified container and outputs the mountpoint
`

	_mountCommand = &cobra.Command{
		Use:   "mount [flags] [CONTAINER]",
		Short: "Mount a working container's root filesystem",
		Long:  mountDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			mountCommand.InputArgs = args
			mountCommand.GlobalFlags = MainGlobalOpts
			mountCommand.Remote = remoteclient
			return mountCmd(&mountCommand)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			return checkAllAndLatest(cmd, args, true)
		},
	}
)

func init() {
	mountCommand.Command = _mountCommand
	mountCommand.SetHelpTemplate(HelpTemplate())
	mountCommand.SetUsageTemplate(UsageTemplate())
	flags := mountCommand.Flags()
	flags.BoolVarP(&mountCommand.All, "all", "a", false, "Mount all containers")
	flags.StringVar(&mountCommand.Format, "format", "", "Change the output format to Go template")
	flags.BoolVarP(&mountCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.BoolVar(&mountCommand.NoTrunc, "notruncate", false, "Do not truncate output")

	markFlagHiddenForRemoteClient("latest", flags)
}

// jsonMountPoint stores info about each container
type jsonMountPoint struct {
	ID         string   `json:"id"`
	Names      []string `json:"names"`
	MountPoint string   `json:"mountpoint"`
}

func mountCmd(c *cliconfig.MountValues) error {
	runtime, err := libpodruntime.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	if os.Geteuid() != 0 {
		rtc, err := runtime.GetConfig()
		if err != nil {
			return err
		}
		if driver := rtc.StorageConfig.GraphDriverName; driver != "vfs" {
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

	if c.All && c.Latest {
		return errors.Errorf("--all and --latest cannot be used together")
	}

	mountContainers, err := getAllOrLatestContainers(&c.PodmanCommand, runtime, -1, "all")
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

	json := c.Format == of.JSONString
	if !formats[c.Format] {
		return errors.Errorf("%q is not a supported format", c.Format)
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

			if c.NoTrunc {
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
