package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	// Kernel namespaces shared by default within a pod
	DefaultKernelNamespaces = "cgroup,ipc,net,uts"
)

var podCreateDescription = "Creates a new empty pod. The pod ID is then" +
	" printed to stdout. You can then start it at any time with the" +
	" podman pod start <pod_id> command. The pod will be created with the" +
	" initial state 'created'."

var podCreateFlags = []cli.Flag{
	cli.StringFlag{
		Name:  "cgroup-parent",
		Usage: "Set parent cgroup for the pod",
	},
	cli.BoolTFlag{
		Name:  "infra",
		Usage: "Create an infra container associated with the pod to share namespaces with",
	},
	cli.StringFlag{
		Name:  "infra-image",
		Usage: "The image of the infra container to associate with the pod",
		Value: libpod.DefaultInfraImage,
	},
	cli.StringFlag{
		Name:  "infra-command",
		Usage: "The command to run on the infra container when the pod is started",
		Value: libpod.DefaultInfraCommand,
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
	cli.StringFlag{
		Name:  "share",
		Usage: "A comma delimited list of kernel namespaces the pod will share",
		Value: DefaultKernelNamespaces,
	},
}

var podCreateCommand = cli.Command{
	Name:                   "create",
	Usage:                  "Create a new empty pod",
	Description:            podCreateDescription,
	Flags:                  podCreateFlags,
	Action:                 podCreateCmd,
	SkipArgReorder:         true,
	UseShortOptionHandling: true,
	OnUsageError:           usageErrorHandler,
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
	if !c.BoolT("infra") && c.IsSet("share") && c.String("share") != "none" && c.String("share") != "" {
		return errors.Errorf("You cannot share kernel namespaces on the pod level without an infra container")
	}

	if c.IsSet("cgroup-parent") {
		options = append(options, libpod.WithPodCgroupParent(c.String("cgroup-parent")))
	}

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

	if c.BoolT("infra") {
		options = append(options, libpod.WithInfraContainer())
		nsOptions, err := shared.GetNamespaceOptions(strings.Split(c.String("share"), ","))
		if err != nil {
			return err
		}
		options = append(options, nsOptions...)
	}

	// always have containers use pod cgroups
	// User Opt out is not yet supported
	options = append(options, libpod.WithPodCgroups())

	ctx := getContext()
	pod, err := runtime.NewPod(ctx, options...)
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
