package system

import (
	"os"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	unshareOptions     = entities.SystemUnshareOptions{}
	unshareDescription = "Runs a command in a modified user namespace."
	unshareCommand     = &cobra.Command{
		Annotations:       map[string]string{registry.EngineMode: registry.ABIMode},
		Use:               "unshare [options] [COMMAND [ARG...]]",
		Short:             "Run a command in a modified user namespace",
		Long:              unshareDescription,
		RunE:              unshare,
		ValidArgsFunction: completion.AutocompleteDefault,
		Example: `podman unshare id
  podman unshare cat /proc/self/uid_map,
  podman unshare podman-script.sh`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: unshareCommand,
	})
	flags := unshareCommand.Flags()
	flags.SetInterspersed(false)
	flags.BoolVar(&unshareOptions.RootlessCNI, "rootless-cni", false, "Join the rootless network namespace used for CNI networking")
}

func unshare(cmd *cobra.Command, args []string) error {
	if isRootless := rootless.IsRootless(); !isRootless {
		return errors.Errorf("please use unshare with rootless")
	}
	// exec the specified command, if there is one
	if len(args) < 1 {
		// try to exec the shell, if one's set
		shell, shellSet := os.LookupEnv("SHELL")
		if !shellSet {
			return errors.Errorf("no command specified and no $SHELL specified")
		}
		args = []string{shell}
	}

	return registry.ContainerEngine().Unshare(registry.Context(), args, unshareOptions)
}
