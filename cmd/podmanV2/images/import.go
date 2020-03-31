package images

import (
	"context"
	"fmt"

	"github.com/containers/libpod/cmd/podmanV2/parse"
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	importDescription = `Create a container image from the contents of the specified tarball (.tar, .tar.gz, .tgz, .bzip, .tar.xz, .txz).

  Note remote tar balls can be specified, via web address.
  Optionally tag the image. You can specify the instructions using the --change option.`
	importCommand = &cobra.Command{
		Use:               "import [flags] PATH [REFERENCE]",
		Short:             "Import a tarball to create a filesystem image",
		Long:              importDescription,
		RunE:              importCon,
		PersistentPreRunE: preRunE,
		Example: `podman import http://example.com/ctr.tar url-image
  cat ctr.tar | podman -q import --message "importing the ctr.tar tarball" - image-imported
  cat ctr.tar | podman import -`,
	}
)

var (
	importOpts entities.ImageImportOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: importCommand,
	})

	importCommand.SetHelpTemplate(registry.HelpTemplate())
	importCommand.SetUsageTemplate(registry.UsageTemplate())
	flags := importCommand.Flags()
	flags.StringArrayVarP(&importOpts.Changes, "change", "c", []string{}, "Apply the following possible instructions to the created image (default []): CMD | ENTRYPOINT | ENV | EXPOSE | LABEL | STOPSIGNAL | USER | VOLUME | WORKDIR")
	flags.StringVarP(&importOpts.Message, "message", "m", "", "Set commit message for imported image")
	flags.BoolVarP(&importOpts.Quiet, "quiet", "q", false, "Suppress output")
}

func importCon(cmd *cobra.Command, args []string) error {
	var (
		source    string
		reference string
	)
	switch len(args) {
	case 0:
		return errors.Errorf("need to give the path to the tarball, or must specify a tarball of '-' for stdin")
	case 1:
		source = args[0]
	case 2:
		source = args[0]
		// TODO when save is merged, we need to process reference
		// like it is done in there or we end up with docker.io prepends
		// instead of the localhost ones
		reference = args[1]
	default:
		return errors.Errorf("too many arguments. Usage TARBALL [REFERENCE]")
	}
	errFileName := parse.ValidateFileName(source)
	errURL := parse.ValidURL(source)
	if errURL == nil {
		importOpts.SourceIsURL = true
	}
	if errFileName != nil && errURL != nil {
		return multierror.Append(errFileName, errURL)
	}

	importOpts.Source = source
	importOpts.Reference = reference

	response, err := registry.ImageEngine().Import(context.Background(), importOpts)
	if err != nil {
		return err
	}
	fmt.Println(response.Id)
	return nil
}
