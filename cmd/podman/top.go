package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
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
		Use:   "top [flags] CONTAINER [FORMAT-DESCRIPTORS]",
		Short: "Display the running processes of a container",
		Long:  topDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			topCommand.InputArgs = args
			topCommand.GlobalFlags = MainGlobalOpts
			return topCmd(&topCommand)
		},
		Example: `podman top ctrID
  podman top --latest
  podman top ctrID pid seccomp args %C`,
	}
)

func init() {
	topCommand.Command = _topCommand
	topCommand.SetHelpTemplate(HelpTemplate())
	topCommand.SetUsageTemplate(UsageTemplate())
	flags := topCommand.Flags()
	flags.BoolVar(&topCommand.ListDescriptors, "list-descriptors", false, "")
	flags.MarkHidden("list-descriptors")
	flags.BoolVarP(&topCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	markFlagHiddenForRemoteClient("latest", flags)
}

func topCmd(c *cliconfig.TopValues) error {
	var container *libpod.Container
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

	rootless.SetSkipStorageSetup(true)
	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	var descriptors []string
	if c.Latest {
		descriptors = args
		container, err = runtime.GetLatestContainer()
	} else {
		descriptors = args[1:]
		container, err = runtime.LookupContainer(args[0])
	}

	if err != nil {
		return errors.Wrapf(err, "unable to lookup requested container")
	}

	conStat, err := container.State()
	if err != nil {
		return errors.Wrapf(err, "unable to look up state for %s", args[0])
	}
	if conStat != libpod.ContainerStateRunning {
		return errors.Errorf("top can only be used on running containers")
	}

	pid, err := container.PID()
	if err != nil {
		return err
	}
	became, ret, err := rootless.JoinNS(uint(pid), 0)
	if err != nil {
		return err
	}
	if became {
		os.Exit(ret)
	}
	psOutput, err := container.GetContainerPidInformation(descriptors)
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
