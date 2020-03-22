package pods

import (
	"context"
	"fmt"

	"github.com/containers/libpod/cmd/podmanV2/parse"
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/cmd/podmanV2/utils"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	podRmDescription = fmt.Sprintf(`podman rm will remove one or more stopped pods and their containers from the host.

  The pod name or ID can be used.  A pod with containers will not be removed without --force. If --force is specified, all containers will be stopped, then removed.`)
	rmCommand = &cobra.Command{
		Use:   "rm [flags] POD [POD...]",
		Short: "Remove one or more pods",
		Long:  podRmDescription,
		RunE:  rm,
		Args: func(cmd *cobra.Command, args []string) error {
			return parse.CheckAllLatestAndCIDFile(cmd, args, false, false)
		},
		Example: `podman pod rm mywebserverpod
  podman pod rm -f 860a4b23
  podman pod rm -f -a`,
	}
)

var (
	rmOptions = entities.PodRmOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: rmCommand,
		Parent:  podCmd,
	})

	flags := rmCommand.Flags()
	flags.BoolVarP(&rmOptions.All, "all", "a", false, "Restart all running pods")
	flags.BoolVarP(&rmOptions.Force, "force", "f", false, "Force removal of a running pod by first stopping all containers, then removing all containers in the pod.  The default is false")
	flags.BoolVarP(&rmOptions.Ignore, "ignore", "i", false, "Ignore errors when a specified pod is missing")
	flags.BoolVarP(&rmOptions.Latest, "latest", "l", false, "Restart the latest pod podman is aware of")
	if registry.IsRemote() {
		_ = flags.MarkHidden("latest")
		_ = flags.MarkHidden("ignore")
	}
}

func rm(cmd *cobra.Command, args []string) error {
	var (
		errs utils.OutputErrors
	)
	responses, err := registry.ContainerEngine().PodRm(context.Background(), args, rmOptions)
	if err != nil {
		return err
	}
	// in the cli, first we print out all the successful attempts
	for _, r := range responses {
		if r.Err == nil {
			fmt.Println(r.Id)
		} else {
			errs = append(errs, r.Err)
		}
	}
	return errs.PrintErrors()
}
