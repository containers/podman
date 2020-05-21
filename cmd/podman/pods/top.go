package pods

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
)

var (
	topDescription = `Specify format descriptors to alter the output.

  You may run "podman pod top -l pid pcpu seccomp" to print the process ID, the CPU percentage and the seccomp mode of each process of the latest pod.`

	topOptions = entities.PodTopOptions{}

	topCommand = &cobra.Command{
		Use:   "top [flags] POD [FORMAT-DESCRIPTORS|ARGS]",
		Short: "Display the running processes of containers in a pod",
		Long:  topDescription,
		RunE:  top,
		Args:  cobra.ArbitraryArgs,
		Example: `podman pod top podID
podman pod top --latest
podman pod top podID pid seccomp args %C
podman pod top podID -eo user,pid,comm`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: topCommand,
		Parent:  podCmd,
	})

	descriptors, err := util.GetContainerPidInformationDescriptors()
	if err == nil {
		topDescription = fmt.Sprintf("%s\n\n  Format Descriptors:\n    %s", topDescription, strings.Join(descriptors, ","))
		topCommand.Long = topDescription
	}

	flags := topCommand.Flags()
	flags.SetInterspersed(false)
	flags.BoolVar(&topOptions.ListDescriptors, "list-descriptors", false, "")
	flags.BoolVarP(&topOptions.Latest, "latest", "l", false, "Act on the latest container podman is aware of")

	_ = flags.MarkHidden("list-descriptors") // meant only for bash completion
	if registry.IsRemote() {
		_ = flags.MarkHidden("latest")
	}
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
		return errors.Errorf("you must provide the name or id of a running pod")
	}

	if topOptions.Latest {
		topOptions.Descriptors = args
	} else {
		topOptions.NameOrID = args[0]
		topOptions.Descriptors = args[1:]
	}

	topResponse, err := registry.ContainerEngine().PodTop(context.Background(), topOptions)
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
