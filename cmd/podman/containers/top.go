package containers

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	topDescription = `Similar to system "top" command.

  Specify format descriptors to alter the output.

  Running "podman top -l pid pcpu seccomp" will print the process ID, the CPU percentage and the seccomp mode of each process of the latest container.`

	topOptions = entities.TopOptions{}

	topCommand = &cobra.Command{
		Use:   "top [flags] CONTAINER [FORMAT-DESCRIPTORS|ARGS]",
		Short: "Display the running processes of a container",
		Long:  topDescription,
		RunE:  top,
		Args:  cobra.ArbitraryArgs,
		Example: `podman top ctrID
podman top --latest
podman top ctrID pid seccomp args %C
podman top ctrID -eo user,pid,comm`,
	}

	containerTopCommand = &cobra.Command{
		Use:   topCommand.Use,
		Short: topCommand.Short,
		Long:  topCommand.Long,
		RunE:  topCommand.RunE,
		Example: `podman container top ctrID
podman container top --latest
podman container top ctrID pid seccomp args %C
podman container top ctrID -eo user,pid,comm`,
	}
)

func topFlags(flags *pflag.FlagSet) {
	flags.SetInterspersed(false)
	flags.BoolVar(&topOptions.ListDescriptors, "list-descriptors", false, "")
	flags.BoolVarP(&topOptions.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	_ = flags.MarkHidden("list-descriptors") // meant only for bash completion
	if registry.IsRemote() {
		_ = flags.MarkHidden("latest")
	}
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: topCommand,
	})
	flags := topCommand.Flags()
	topFlags(flags)

	descriptors, err := util.GetContainerPidInformationDescriptors()
	if err == nil {
		topDescription = fmt.Sprintf("%s\n\n  Format Descriptors:\n    %s", topDescription, strings.Join(descriptors, ","))
		topCommand.Long = topDescription
	}

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: containerTopCommand,
		Parent:  containerCmd,
	})
	containerTopFlags := containerTopCommand.Flags()
	topFlags(containerTopFlags)
}

func top(cmd *cobra.Command, args []string) error {
	if topOptions.ListDescriptors {
		descriptors, err := util.GetContainerPidInformationDescriptors()
		if err != nil {
			return err
		}
		fmt.Println(strings.Join(descriptors, "\n"))
		return nil
	}

	if len(args) < 1 && !topOptions.Latest {
		return errors.Errorf("you must provide the name or id of a running container")
	}

	if topOptions.Latest {
		topOptions.Descriptors = args
	} else {
		topOptions.NameOrID = args[0]
		topOptions.Descriptors = args[1:]
	}

	topResponse, err := registry.ContainerEngine().ContainerTop(context.Background(), topOptions)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 5, 1, 3, ' ', 0)
	for _, proc := range topResponse.Value {
		if _, err := fmt.Fprintln(w, proc); err != nil {
			return err
		}
	}
	return w.Flush()
}
