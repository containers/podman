package main

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/cmd/podman/libpodruntime"
	"github.com/urfave/cli"
)

var (
	podStartFlags       = []cli.Flag{}
	podStartDescription = `
   podman pod start

   Starts one or more pods.  The pod name or ID can be used.
`

	podStartCommand = cli.Command{
		Name:                   "start",
		Usage:                  "Start one or more pods",
		Description:            podStartDescription,
		Flags:                  podStartFlags,
		Action:                 podStartCmd,
		ArgsUsage:              "POD-NAME [POD-NAME ...]",
		UseShortOptionHandling: true,
	}
)

func podStartCmd(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 {
		return errors.Errorf("you must provide at least one pod name or id")
	}

	if err := validateFlags(c, startFlags); err != nil {
		return err
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	var lastError error
	for _, pod_str := range args {
		pod, err := runtime.LookupPod(pod_str)
		if err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "unable to find pod %s", pod_str)
			continue
		}

		ctr_errs, err := pod.Start(getContext())
		if err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "unable to start pod %q", pod_str)
			continue
		} else if ctr_errs != nil {
			for ctr, err := range ctr_errs {
				if lastError != nil {
					fmt.Fprintln(os.Stderr, lastError)
				}
				lastError = errors.Wrapf(err, "unable to start container %q on pod %q", ctr, pod_str)
			}
			continue
		}
		fmt.Println(pod_str)
	}

	return lastError
}
