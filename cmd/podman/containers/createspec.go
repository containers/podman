package containers

import (
	"context"
	"fmt"
	"os"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/containers/podman/v4/pkg/specgenutil"
	"github.com/spf13/cobra"
)

var (
	createSpecDescription = `Creates a new container from the given json file cntaining a filled out specgen`
	createSpecCommand     = &cobra.Command{
		Use:               "createspec [options] FILE",
		Short:             "Create a new container from json",
		Long:              createSpecDescription,
		RunE:              createspec,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman container createspec --cpus=5 ~/Documents/spec.json`,
	}
)

var (
	containerOpts entities.ContainerCreateOptions
	startCtr      bool
)

func createSpecFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	startFlagName := "start"
	flags.BoolVar(&startCtr, startFlagName, false, "start the new container")

	flags.SetInterspersed(false)
	common.DefineCreateDefaults(&containerOpts)
	common.DefineCreateFlags(cmd, &containerOpts, false, false)
	common.DefineNetFlags(cmd)
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: createSpecCommand,
		Parent:  containerCmd,
	})

	createSpecFlags(createSpecCommand)
}

func createspec(cmd *cobra.Command, args []string) error {
	f, err := os.Open(args[0])
	if err != nil {
		return err
	}

	data, err := os.ReadFile(f.Name())
	if err != nil {
		return err
	}

	spec := &specgen.SpecGenerator{}

	err = json.Unmarshal(data, spec)
	if err != nil {
		return err
	}

	err = specgenutil.FillOutSpecGen(spec, &containerOpts, []string{})
	if err != nil {
		return err
	}

	var id string
	rep, err := registry.ContainerEngine().ContainerCreate(context.Background(), spec)
	if err != nil {
		return err
	}

	id = rep.Id

	if startCtr {
		rep, err := registry.ContainerEngine().ContainerStart(context.Background(), []string{rep.Id}, entities.ContainerStartOptions{})
		if err != nil {
			return err
		}
		id = rep[0].Id
	}

	fmt.Println(id)

	return nil
}
