package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	podTopCommand cliconfig.PodTopValues

	podTopDescription = fmt.Sprintf(`Specify format descriptors to alter the output.

  You may run "podman pod top -l pid pcpu seccomp" to print the process ID, the CPU percentage and the seccomp mode of each process of the latest pod.
%s`, getDescriptorString())

	_podTopCommand = &cobra.Command{
		Use:   "top [flags] CONTAINER [FORMAT-DESCRIPTORS]",
		Short: "Display the running processes of containers in a pod",
		Long:  podTopDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			podTopCommand.InputArgs = args
			podTopCommand.GlobalFlags = MainGlobalOpts
			podTopCommand.Remote = remoteclient
			return podTopCmd(&podTopCommand)
		},
		Example: `podman top ctrID
  podman top --latest
  podman top --latest pid seccomp args %C`,
	}
)

func init() {
	podTopCommand.Command = _podTopCommand
	podTopCommand.SetHelpTemplate(HelpTemplate())
	podTopCommand.SetUsageTemplate(UsageTemplate())
	flags := podTopCommand.Flags()
	flags.BoolVarP(&podTopCommand.Latest, "latest,", "l", false, "Act on the latest pod podman is aware of")
	flags.BoolVar(&podTopCommand.ListDescriptors, "list-descriptors", false, "")
	flags.MarkHidden("list-descriptors")

}

func podTopCmd(c *cliconfig.PodTopValues) error {
	var (
		descriptors []string
	)
	args := c.InputArgs

	if c.ListDescriptors {
		descriptors, err := util.GetContainerPidInformationDescriptors()
		if err != nil {
			return err
		}
		fmt.Println(strings.Join(descriptors, "\n"))
		return nil
	}

	if len(args) < 1 && !c.Latest {
		return errors.Errorf("you must provide the name or id of a running pod")
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	if c.Latest {
		descriptors = args
	} else {
		descriptors = args[1:]
	}

	w := tabwriter.NewWriter(os.Stdout, 5, 1, 3, ' ', 0)
	psOutput, err := runtime.PodTop(c, descriptors)
	if err != nil {
		return err
	}
	for _, proc := range psOutput {
		fmt.Fprintln(w, proc)
	}
	w.Flush()
	return nil
}
