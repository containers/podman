package containers

import (
	"context"
	"fmt"
	"strings"

	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/containers/podman/v4/pkg/specgenutil"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/spf13/cobra"
)

var (
	updateDescription = `Updates the cgroup configuration of a given container`

	updateCommand = &cobra.Command{
		Use:               "update [options] CONTAINER",
		Short:             "Update an existing container",
		Long:              updateDescription,
		RunE:              update,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: common.AutocompleteContainers,
		Example:           `podman update --cpus=5 foobar_container`,
	}

	containerUpdateCommand = &cobra.Command{
		Args:              updateCommand.Args,
		Use:               updateCommand.Use,
		Short:             updateCommand.Short,
		Long:              updateCommand.Long,
		RunE:              updateCommand.RunE,
		ValidArgsFunction: updateCommand.ValidArgsFunction,
		Example:           `podman container update --cpus=5 foobar_container`,
	}
)
var (
	updateOpts entities.ContainerCreateOptions
)

func updateFlags(cmd *cobra.Command) {
	common.DefineCreateDefaults(&updateOpts)
	common.DefineCreateFlags(cmd, &updateOpts, entities.UpdateMode)
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: updateCommand,
	})
	updateFlags(updateCommand)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerUpdateCommand,
		Parent:  containerCmd,
	})
	updateFlags(containerUpdateCommand)
}

func update(cmd *cobra.Command, args []string) error {
	var err error
	// use a specgen since this is the easiest way to hold resource info
	s := &specgen.SpecGenerator{}
	s.ResourceLimits = &specs.LinuxResources{}

	err = createOrUpdateFlags(cmd, &updateOpts)
	if err != nil {
		return err
	}

	// we need to pass the whole specgen since throttle devices are parsed later due to cross compat.
	s.ResourceLimits, err = specgenutil.GetResources(s, &updateOpts)
	if err != nil {
		return err
	}

	opts := &entities.ContainerUpdateOptions{
		NameOrID: strings.TrimPrefix(args[0], "/"),
		Specgen:  s,
	}
	rep, err := registry.ContainerEngine().ContainerUpdate(context.Background(), opts)
	if err != nil {
		return err
	}
	fmt.Println(rep)
	return nil
}
