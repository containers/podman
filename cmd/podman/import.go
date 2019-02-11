package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	importCommand cliconfig.ImportValues

	importDescription = `Create a container image from the contents of the specified tarball (.tar, .tar.gz, .tgz, .bzip, .tar.xz, .txz).
	 Note remote tar balls can be specified, via web address.
	 Optionally tag the image. You can specify the instructions using the --change option.
	`
	_importCommand = &cobra.Command{
		Use:   "import",
		Short: "Import a tarball to create a filesystem image",
		Long:  importDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			importCommand.InputArgs = args
			importCommand.GlobalFlags = MainGlobalOpts
			return importCmd(&importCommand)
		},
		Example: "TARBALL [REFERENCE]",
	}
)

func init() {
	importCommand.Command = _importCommand
	flags := importCommand.Flags()
	flags.StringSliceVarP(&importCommand.Change, "change", "c", []string{}, "Apply the following possible instructions to the created image (default []): CMD | ENTRYPOINT | ENV | EXPOSE | LABEL | STOPSIGNAL | USER | VOLUME | WORKDIR")
	flags.StringVarP(&importCommand.Message, "message", "m", "", "Set commit message for imported image")
	flags.BoolVarP(&importCommand.Quiet, "quiet", "q", false, "Suppress output")

}

func importCmd(c *cliconfig.ImportValues) error {
	runtime, err := adapter.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

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

	if err := validateFileName(source); err != nil {
		return err
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
