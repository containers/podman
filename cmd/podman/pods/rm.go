package pods

import (
	"context"
	"fmt"
	"strings"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/utils"
	"github.com/containers/podman/v3/cmd/podman/validate"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// allows for splitting API and CLI-only options
type podRmOptionsWrapper struct {
	entities.PodRmOptions

	PodIDFiles []string
}

var (
	rmOptions        = podRmOptionsWrapper{}
	podRmDescription = `podman rm will remove one or more stopped pods and their containers from the host.

  The pod name or ID can be used.  A pod with containers will not be removed without --force. If --force is specified, all containers will be stopped, then removed.`
	rmCommand = &cobra.Command{
		Use:   "rm [options] POD [POD...]",
		Short: "Remove one or more pods",
		Long:  podRmDescription,
		RunE:  rm,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndPodIDFile(cmd, args, false, true)
		},
		ValidArgsFunction: common.AutocompletePods,
		Example: `podman pod rm mywebserverpod
  podman pod rm -f 860a4b23
  podman pod rm -f -a`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmCommand,
		Parent:  podCmd,
	})

	flags := rmCommand.Flags()
	flags.BoolVarP(&rmOptions.All, "all", "a", false, "Remove all running pods")
	flags.BoolVarP(&rmOptions.Force, "force", "f", false, "Force removal of a running pod by first stopping all containers, then removing all containers in the pod.  The default is false")
	flags.BoolVarP(&rmOptions.Ignore, "ignore", "i", false, "Ignore errors when a specified pod is missing")

	podIDFileFlagName := "pod-id-file"
	flags.StringArrayVarP(&rmOptions.PodIDFiles, podIDFileFlagName, "", nil, "Read the pod ID from the file")
	_ = rmCommand.RegisterFlagCompletionFunc(podIDFileFlagName, completion.AutocompleteDefault)

	validate.AddLatestFlag(rmCommand, &rmOptions.Latest)

	if registry.IsRemote() {
		_ = flags.MarkHidden("ignore")
	}
}

func rm(_ *cobra.Command, args []string) error {
	ids, err := common.ReadPodIDFiles(rmOptions.PodIDFiles)
	if err != nil {
		return err
	}
	args = append(args, ids...)
	return removePods(args, rmOptions.PodRmOptions, true)
}

// removePods removes the specified pods (names or IDs).  Allows for sharing
// pod-removal logic across commands.
func removePods(namesOrIDs []string, rmOptions entities.PodRmOptions, printIDs bool) error {
	var errs utils.OutputErrors

	responses, err := registry.ContainerEngine().PodRm(context.Background(), namesOrIDs, rmOptions)
	if err != nil {
		setExitCode(err)
		return err
	}

	// in the cli, first we print out all the successful attempts
	for _, r := range responses {
		if r.Err == nil {
			if printIDs {
				fmt.Println(r.Id)
			}
		} else {
			setExitCode(r.Err)
			errs = append(errs, r.Err)
		}
	}
	return errs.PrintErrors()
}

func setExitCode(err error) {
	cause := errors.Cause(err)
	switch {
	case cause == define.ErrNoSuchPod:
		registry.SetExitCode(1)
	case strings.Contains(cause.Error(), define.ErrNoSuchPod.Error()):
		registry.SetExitCode(1)
	}
}
