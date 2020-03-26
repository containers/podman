package containers

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/psgo"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	topDescription = fmt.Sprintf(`Similar to system "top" command.

  Specify format descriptors to alter the output.

  Running "podman top -l pid pcpu seccomp" will print the process ID, the CPU percentage and the seccomp mode of each process of the latest container.
  Format Descriptors:
    %s`, strings.Join(psgo.ListDescriptors(), ","))

	topOptions = entities.TopOptions{}

	topCommand = &cobra.Command{
		Use:               "top [flags] CONTAINER [FORMAT-DESCRIPTORS|ARGS]",
		Short:             "Display the running processes of a container",
		Long:              topDescription,
		PersistentPreRunE: preRunE,
		RunE:              top,
		Args:              cobra.ArbitraryArgs,
		Example: `podman top ctrID
podman top --latest
podman top ctrID pid seccomp args %C
podman top ctrID -eo user,pid,comm`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: topCommand,
	})

	topCommand.SetHelpTemplate(registry.HelpTemplate())
	topCommand.SetUsageTemplate(registry.UsageTemplate())

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
		fmt.Println(strings.Join(psgo.ListDescriptors(), "\n"))
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
