package main

import (
	"os"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/adapter"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	ociManifestDir  = "oci-dir"
	v2s2ManifestDir = "docker-dir"
)

var (
	saveCommand     cliconfig.SaveValues
	saveDescription = `
	Save an image to docker-archive or oci-archive on the local machine.
	Default is docker-archive`

	_saveCommand = &cobra.Command{
		Use:   "save",
		Short: "Save image to an archive",
		Long:  saveDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			saveCommand.InputArgs = args
			saveCommand.GlobalFlags = MainGlobalOpts
			return saveCmd(&saveCommand)
		},
		Example: `podman save --quiet -o myimage.tar imageID
  podman save --format docker-dir -o ubuntu-dir ubuntu
  podman save > alpine-all.tar alpine:latest`,
	}
)

func init() {
	saveCommand.Command = _saveCommand
	saveCommand.SetUsageTemplate(UsageTemplate())
	flags := saveCommand.Flags()
	flags.BoolVar(&saveCommand.Compress, "compress", false, "Compress tarball image layers when saving to a directory using the 'dir' transport. (default is same compression type as source)")
	flags.StringVar(&saveCommand.Format, "format", "docker-archive", "Save image to oci-archive, oci-dir (directory with oci manifest type), docker-dir (directory with v2s2 manifest type)")
	flags.StringVarP(&saveCommand.Output, "output", "o", "/dev/stdout", "Write to a file, default is STDOUT")
	flags.BoolVarP(&saveCommand.Quiet, "quiet", "q", false, "Suppress the output")
}

// saveCmd saves the image to either docker-archive or oci
func saveCmd(c *cliconfig.SaveValues) error {
	args := c.InputArgs
	if len(args) == 0 {
		return errors.Errorf("need at least 1 argument")
	}

	runtime, err := adapter.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not create runtime")
	}
	defer runtime.Shutdown(false)

	if c.Flag("compress").Changed && (c.Format != ociManifestDir && c.Format != v2s2ManifestDir && c.Format == "") {
		return errors.Errorf("--compress can only be set when --format is either 'oci-dir' or 'docker-dir'")
	}

	output := c.Output
	if output == "/dev/stdout" {
		fi := os.Stdout
		if logrus.IsTerminal(fi) {
			return errors.Errorf("refusing to save to terminal. Use -o flag or redirect")
		}
	}
	if err := validateFileName(output); err != nil {
		return err
	}
	return runtime.SaveImage(getContext(), c)
}
