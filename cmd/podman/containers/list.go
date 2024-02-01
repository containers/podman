package containers

import (
	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/spf13/cobra"
)

var (
	// podman container _list_
	listCmd = &cobra.Command{
		Use:               "list [options]",
		Aliases:           []string{"ls"},
		Args:              validate.NoArgs,
		Short:             "List containers",
		Long:              "Prints out information about the containers",
		RunE:              ps,
		ValidArgsFunction: completion.AutocompleteNone,
		Example: `podman container list -a
  podman container list -a --format "{{.ID}}  {{.Image}}  {{.Labels}}  {{.Mounts}}"
  podman container list --size --sort names`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: listCmd,
		Parent:  containerCmd,
	})
	listFlagSet(listCmd)
	validate.AddLatestFlag(listCmd, &listOpts.Latest)
}
