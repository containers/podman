package volumes

import (
	"context"
	"fmt"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v5/cmd/podman/parse"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	createDescription = `If using the default driver, "local", the volume will be created on the host in the volumes directory under container storage.`

	createCommand = &cobra.Command{
		Use:               "create [options] [NAME]",
		Args:              cobra.MaximumNArgs(1),
		Short:             "Create a new volume",
		Long:              createDescription,
		RunE:              create,
		ValidArgsFunction: completion.AutocompleteNone,
		Example: `podman volume create myvol
  podman volume create
  podman volume create --label foo=bar myvol`,
	}
)

var (
	createOpts = entities.VolumeCreateOptions{}
	opts       = struct {
		Label  []string
		Opts   []string
		Ignore bool
	}{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: createCommand,
		Parent:  volumeCmd,
	})
	flags := createCommand.Flags()

	driverFlagName := "driver"
	flags.StringVarP(&createOpts.Driver, driverFlagName, "d", "local", "Specify volume driver name")
	_ = createCommand.RegisterFlagCompletionFunc(driverFlagName, completion.AutocompleteNone)

	labelFlagName := "label"
	flags.StringArrayVarP(&opts.Label, labelFlagName, "l", []string{}, "Set metadata for a volume (default [])")
	_ = createCommand.RegisterFlagCompletionFunc(labelFlagName, completion.AutocompleteNone)

	optFlagName := "opt"
	flags.StringArrayVarP(&opts.Opts, optFlagName, "o", []string{}, "Set driver specific options (default [])")
	_ = createCommand.RegisterFlagCompletionFunc(optFlagName, completion.AutocompleteNone)

	ignoreFlagName := "ignore"
	flags.BoolVar(&opts.Ignore, ignoreFlagName, false, "Don't fail if volume already exists")
}

func create(cmd *cobra.Command, args []string) error {
	var (
		err error
	)
	if len(args) > 0 {
		createOpts.Name = args[0]
	}

	createOpts.IgnoreIfExists = opts.Ignore

	createOpts.Label, err = parse.GetAllLabels([]string{}, opts.Label)
	if err != nil {
		return fmt.Errorf("unable to process labels: %w", err)
	}
	createOpts.Options, err = parse.GetAllLabels([]string{}, opts.Opts)
	if err != nil {
		return fmt.Errorf("unable to process options: %w", err)
	}
	response, err := registry.ContainerEngine().VolumeCreate(context.Background(), createOpts)
	if err != nil {
		return err
	}
	fmt.Println(response.IDOrName)
	return nil
}
