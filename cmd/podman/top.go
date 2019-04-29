package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func getDescriptorString() string {
	descriptors, err := libpod.GetContainerPidInformationDescriptors()
	if err == nil {
		return fmt.Sprintf(`
  Format Descriptors:
    %s`, strings.Join(descriptors, ","))
	}
	return ""
}

var (
	topCommand     cliconfig.TopValues
	topDescription = fmt.Sprintf(`Similar to system "top" command.

  Specify format descriptors to alter the output.

  Running "podman top -l pid pcpu seccomp" will print the process ID, the CPU percentage and the seccomp mode of each process of the latest container.
%s`, getDescriptorString())

	_topCommand = &cobra.Command{
		Use:   "top [flags] CONTAINER [FORMAT-DESCRIPTORS|ARGS]",
		Short: "Display the running processes of a container",
		Long:  topDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			topCommand.InputArgs = args
			topCommand.GlobalFlags = MainGlobalOpts
			topCommand.Remote = remoteclient
			return topCmd(&topCommand)
		},
		Args: cobra.ArbitraryArgs,
		Example: `podman top ctrID
podman top --latest
podman top ctrID pid seccomp args %C
podman top ctrID -eo user,pid,comm`,
	}
)

func init() {
	topCommand.Command = _topCommand
	topCommand.SetHelpTemplate(HelpTemplate())
	topCommand.SetUsageTemplate(UsageTemplate())
	flags := topCommand.Flags()
	flags.SetInterspersed(false)
	flags.BoolVar(&topCommand.ListDescriptors, "list-descriptors", false, "")
	flags.MarkHidden("list-descriptors")
	flags.BoolVarP(&topCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	markFlagHiddenForRemoteClient("latest", flags)
}

func topCmd(c *cliconfig.TopValues) error {
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
		return errors.Errorf("you must provide the name or id of a running container")
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	psOutput, err := runtime.Top(c)
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
