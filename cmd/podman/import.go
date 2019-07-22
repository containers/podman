package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/shared/parse"
	"github.com/containers/libpod/pkg/adapter"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	importCommand cliconfig.ImportValues

	importDescription = `Create a container image from the contents of the specified tarball (.tar, .tar.gz, .tgz, .bzip, .tar.xz, .txz).

  Note remote tar balls can be specified, via web address.
  Optionally tag the image. You can specify the instructions using the --change option.`
	_importCommand = &cobra.Command{
		Use:   "import [flags] PATH [REFERENCE]",
		Short: "Import a tarball to create a filesystem image",
		Long:  importDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			importCommand.InputArgs = args
			importCommand.GlobalFlags = MainGlobalOpts
			importCommand.Remote = remoteclient
			return importCmd(&importCommand)
		},
		Example: `podman import http://example.com/ctr.tar url-image
  cat ctr.tar | podman -q import --message "importing the ctr.tar tarball" - image-imported
  cat ctr.tar | podman import -`,
	}
)

func init() {
	importCommand.Command = _importCommand
	importCommand.SetHelpTemplate(HelpTemplate())
	importCommand.SetUsageTemplate(UsageTemplate())
	flags := importCommand.Flags()
	flags.StringSliceVarP(&importCommand.Change, "change", "c", []string{}, "Apply the following possible instructions to the created image (default []): CMD | ENTRYPOINT | ENV | EXPOSE | LABEL | STOPSIGNAL | USER | VOLUME | WORKDIR")
	flags.StringVarP(&importCommand.Message, "message", "m", "", "Set commit message for imported image")
	flags.BoolVarP(&importCommand.Quiet, "quiet", "q", false, "Suppress output")

}

func importCmd(c *cliconfig.ImportValues) error {
	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.DeferredShutdown(false)

	var (
		source    string
		reference string
	)

	args := c.InputArgs
	switch len(args) {
	case 0:
		return errors.Errorf("need to give the path to the tarball, or must specify a tarball of '-' for stdin")
	case 1:
		source = args[0]
	case 2:
		source = args[0]
		reference = args[1]
	default:
		return errors.Errorf("too many arguments. Usage TARBALL [REFERENCE]")
	}

	errFileName := parse.ValidateFileName(source)
	errURL := parse.ValidURL(source)

	if errFileName != nil && errURL != nil {
		return multierror.Append(errFileName, errURL)
	}

	quiet := c.Quiet
	if runtime.Remote {
		quiet = false
	}
	iid, err := runtime.Import(getContext(), source, reference, c.StringSlice("change"), c.String("message"), quiet)
	if err == nil {
		fmt.Println(iid)
	}
	return err
}
