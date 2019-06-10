package main

import (
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	// Kernel namespaces shared by default within a pod

	podCreateCommand cliconfig.PodCreateValues

	podCreateDescription = `After creating the pod, the pod ID is printed to stdout.

  You can then start it at any time with the  podman pod start <pod_id> command. The pod will be created with the initial state 'created'.`

	_podCreateCommand = &cobra.Command{
		Use:   "create",
		Args:  noSubArgs,
		Short: "Create a new empty pod",
		Long:  podCreateDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			podCreateCommand.InputArgs = args
			podCreateCommand.GlobalFlags = MainGlobalOpts
			podCreateCommand.Remote = remoteclient
			return podCreateCmd(&podCreateCommand)
		},
	}
)

func init() {
	podCreateCommand.Command = _podCreateCommand
	podCreateCommand.SetHelpTemplate(HelpTemplate())
	podCreateCommand.SetUsageTemplate(UsageTemplate())
	flags := podCreateCommand.Flags()
	flags.SetInterspersed(false)

	flags.StringVar(&podCreateCommand.CgroupParent, "cgroup-parent", "", "Set parent cgroup for the pod")
	flags.BoolVar(&podCreateCommand.Infra, "infra", true, "Create an infra container associated with the pod to share namespaces with")
	flags.StringVar(&podCreateCommand.InfraImage, "infra-image", libpod.DefaultInfraImage, "The image of the infra container to associate with the pod")
	flags.StringVar(&podCreateCommand.InfraCommand, "infra-command", libpod.DefaultInfraCommand, "The command to run on the infra container when the pod is started")
	flags.StringSliceVar(&podCreateCommand.LabelFile, "label-file", []string{}, "Read in a line delimited file of labels")
	flags.StringSliceVarP(&podCreateCommand.Labels, "label", "l", []string{}, "Set metadata on pod (default [])")
	flags.StringVarP(&podCreateCommand.Name, "name", "n", "", "Assign a name to the pod")
	flags.StringVar(&podCreateCommand.PodIDFile, "pod-id-file", "", "Write the pod ID to the file")
	flags.StringSliceVarP(&podCreateCommand.Publish, "publish", "p", []string{}, "Publish a container's port, or a range of ports, to the host (default [])")
	flags.StringVar(&podCreateCommand.Share, "share", shared.DefaultKernelNamespaces, "A comma delimited list of kernel namespaces the pod will share")

}

func podCreateCmd(c *cliconfig.PodCreateValues) error {
	var (
		err       error
		podIdFile *os.File
	)

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	if len(c.Publish) > 0 {
		if !c.Infra {
			return errors.Errorf("you must have an infra container to publish port bindings to the host")
		}
	}

	if !c.Infra && c.Flag("share").Changed && c.Share != "none" && c.Share != "" {
		return errors.Errorf("You cannot share kernel namespaces on the pod level without an infra container")
	}
	if c.Flag("pod-id-file").Changed && os.Geteuid() == 0 {
		podIdFile, err = libpod.OpenExclusiveFile(c.PodIDFile)
		if err != nil && os.IsExist(err) {
			return errors.Errorf("pod id file exists. Ensure another pod is not using it or delete %s", c.PodIDFile)
		}
		if err != nil {
			return errors.Errorf("error opening pod-id-file %s", c.PodIDFile)
		}
		defer podIdFile.Close()
		defer podIdFile.Sync()
	}

	labels, err := shared.GetAllLabels(c.LabelFile, c.Labels)
	if err != nil {
		return errors.Wrapf(err, "unable to process labels")
	}

	podID, err := runtime.CreatePod(getContext(), c, labels)
	if err != nil {
		return errors.Wrapf(err, "unable to create pod")
	}
	if podIdFile != nil {
		_, err = podIdFile.WriteString(podID)
		if err != nil {
			logrus.Error(err)
		}
	}
	fmt.Printf("%s\n", podID)
	return nil
}
