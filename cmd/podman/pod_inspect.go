package main

import (
	"encoding/json"

	"fmt"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	podInspectFlags = []cli.Flag{
		LatestPodFlag,
	}
	podInspectDescription = "display the configuration for a pod by name or id"
	podInspectCommand     = cli.Command{
		Name:                   "inspect",
		Usage:                  "displays a pod configuration",
		Description:            podInspectDescription,
		Flags:                  podInspectFlags,
		Action:                 podInspectCmd,
		UseShortOptionHandling: true,
		ArgsUsage:              "[POD_NAME_OR_ID]",
		OnUsageError:           usageErrorHandler,
	}
)

func podInspectCmd(c *cli.Context) error {
	var (
		pod *libpod.Pod
	)
	if err := checkMutuallyExclusiveFlags(c); err != nil {
		return err
	}
	args := c.Args()
	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	if c.Bool("latest") {
		pod, err = runtime.GetLatestPod()
		if err != nil {
			return errors.Wrapf(err, "unable to get latest pod")
		}
	} else {
		pod, err = runtime.LookupPod(args[0])
		if err != nil {
			return err
		}
	}

	podInspectData, err := pod.Inspect()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(&podInspectData, "", "     ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
