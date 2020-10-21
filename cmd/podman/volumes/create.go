package volumes

import (
	"context"
	"fmt"

	"github.com/containers/podman/v2/cmd/podman/parse"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	createDescription = `If using the default driver, "local", the volume will be created on the host in the volumes directory under container storage.`

	createCommand = &cobra.Command{
		Use:   "create [options] [NAME]",
		Short: "Create a new volume",
		Long:  createDescription,
		RunE:  create,
		Example: `podman volume create myvol
  podman volume create
  podman volume create --label foo=bar myvol`,
	}
)

var (
	createOpts = entities.VolumeCreateOptions{}
	opts       = struct {
		Label []string
		Opts  []string
	}{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: createCommand,
		Parent:  volumeCmd,
	})
	flags := createCommand.Flags()
	flags.StringVar(&createOpts.Driver, "driver", "local", "Specify volume driver name")
	flags.StringSliceVarP(&opts.Label, "label", "l", []string{}, "Set metadata for a volume (default [])")
	flags.StringArrayVarP(&opts.Opts, "opt", "o", []string{}, "Set driver specific options (default [])")
}

func create(cmd *cobra.Command, args []string) error {
	var (
		err error
	)
	if len(args) > 1 {
		return errors.Errorf("too many arguments, create takes at most 1 argument")
	}
	if len(args) > 0 {
		createOpts.Name = args[0]
	}
	createOpts.Label, err = parse.GetAllLabels([]string{}, opts.Label)
	if err != nil {
		return errors.Wrapf(err, "unable to process labels")
	}
	createOpts.Options, err = parse.GetAllLabels([]string{}, opts.Opts)
	if err != nil {
		return errors.Wrapf(err, "unable to process options")
	}
	response, err := registry.ContainerEngine().VolumeCreate(context.Background(), createOpts)
	if err != nil {
		return err
	}
	fmt.Println(response.IDOrName)
	return nil
}
