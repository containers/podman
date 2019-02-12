package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	podTopCommand cliconfig.PodTopValues

	podTopDescription = fmt.Sprintf(`Display the running processes containers in a pod.  Specify format descriptors
to alter the output.  You may run "podman pod top -l pid pcpu seccomp" to print
the process ID, the CPU percentage and the seccomp mode of each process of
the latest pod.
%s
`, getDescriptorString())

	_podTopCommand = &cobra.Command{
		Use:   "top",
		Short: "Display the running processes of containers in a pod",
		Long:  podTopDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			podTopCommand.InputArgs = args
			podTopCommand.GlobalFlags = MainGlobalOpts
			return podTopCmd(&podTopCommand)
		},
		Example: "POD-NAME [format descriptors]",
	}
)

func init() {
	podTopCommand.Command = _podTopCommand
	podTopCommand.SetUsageTemplate(UsageTemplate())
	flags := podTopCommand.Flags()
	flags.BoolVarP(&podTopCommand.Latest, "latest,", "l", false, "Act on the latest pod podman is aware of")
	flags.BoolVar(&podTopCommand.ListDescriptors, "list-descriptors", false, "")
	flags.MarkHidden("list-descriptors")

}

func podTopCmd(c *cliconfig.PodTopValues) error {
	var pod *libpod.Pod
	var err error
	args := c.InputArgs

	if c.ListDescriptors {
		descriptors, err := libpod.GetContainerPidInformationDescriptors()
		if err != nil {
			return err
		}
		fmt.Println(strings.Join(descriptors, "\n"))
		return nil
	}

	if len(args) < 1 && !c.Latest {
		return errors.Errorf("you must provide the name or id of a running pod")
	}

	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	var descriptors []string
	if c.Latest {
		descriptors = args
		pod, err = runtime.GetLatestPod()
	} else {
		descriptors = args[1:]
		pod, err = runtime.LookupPod(args[0])
	}

	if err != nil {
		return errors.Wrapf(err, "unable to lookup requested container")
	}

	podStatus, err := shared.GetPodStatus(pod)
	if err != nil {
		return err
	}
	if podStatus != "Running" {
		return errors.Errorf("pod top can only be used on pods with at least one running container")
	}

	psOutput, err := pod.GetPodPidInformation(descriptors)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 5, 1, 3, ' ', 0)
	for _, proc := range psOutput {
		fmt.Fprintln(w, proc)
	}
	w.Flush()
	return nil
}
