package main

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/cmd/podman/libpodruntime"
	"github.com/projectatomic/libpod/libpod"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var podCreateDescription = "Creates a new empty pod. The pod ID is then" +
	" printed to stdout. You can then start it at any time with the" +
	" podman pod start <pod_id> command. The pod will be created with the" +
	" initial state 'created'."

var podCreateFlags = []cli.Flag{
	cli.BoolTFlag{
		Name:  "cgroup-to-ctr",
		Usage: "Tells containers in this pod to use the cgroup created for the pod",
	},
	cli.StringFlag{
		Name:  "cgroup-parent",
		Usage: "Optional parent cgroup for the pod",
	},
	cli.StringSliceFlag{
		Name:  "label-file",
		Usage: "Read in a line delimited file of labels (default [])",
	},
	cli.StringSliceFlag{
		Name:  "label, l",
		Usage: "Set metadata on pod (default [])",
	},
	cli.StringFlag{
		Name:  "name, n",
		Usage: "Assign a name to the pod",
	},
	cli.StringFlag{
		Name:  "pod-id-file",
		Usage: "Write the pod ID to the file",
	},
}

var podCreateCommand = cli.Command{
	Name:                   "create",
	Usage:                  "create but do not start a pod",
	Description:            podCreateDescription,
	Flags:                  podCreateFlags,
	Action:                 podCreateCmd,
	SkipArgReorder:         true,
	UseShortOptionHandling: true,
}

func podCreateCmd(c *cli.Context) error {
	var options []libpod.PodCreateOption
	var err error

	if err = validateFlags(c, createFlags); err != nil {
		return err
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	if c.IsSet("pod-id-file") {
		if _, err = os.Stat(c.String("pod-id-file")); err == nil {
			return errors.Errorf("pod id file exists. ensure another pod is not using it or delete %s", c.String("pod-id-file"))
		}
		if err = libpod.WriteFile("", c.String("pod-id-file")); err != nil {
			return errors.Wrapf(err, "unable to write pod id file %s", c.String("pod-id-file"))
		}
	}
	// BEGIN GetPodCreateOptions

	// TODO make sure this is correct usage
	if c.IsSet("cgroup-parent") {
		options = append(options, libpod.WithPodCgroupParent(c.String("cgroup-parent")))
	}

	if c.Bool("cgroup-to-ctr") {
		options = append(options, libpod.WithPodCgroups())
	}
	// LABEL VARIABLES
	// TODO make sure this works as expected
	labels, err := getAllLabels(c.StringSlice("label-file"), c.StringSlice("label"))
	if err != nil {
		return errors.Wrapf(err, "unable to process labels")
	}
	if len(labels) != 0 {
		options = append(options, libpod.WithPodLabels(labels))
	}

	if c.IsSet("name") {
		options = append(options, libpod.WithPodName(c.String("name")))
	}

	pod, err := runtime.NewPod(options...)
	if err != nil {
		return err
	}

	if c.IsSet("pod-id-file") {
		err = libpod.WriteFile(pod.ID(), c.String("pod-id-file"))
		if err != nil {
			logrus.Error(err)
		}
	}

	fmt.Printf("%s\n", pod.ID())

	return nil
}
