package main

import (
	"os"
	"strings"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	ociManifestDir  = "oci-dir"
	ociArchive      = "oci-archive"
	v2s2ManifestDir = "docker-dir"
	v2s2Archive     = "docker-archive"
)

var validFormats = []string{ociManifestDir, ociArchive, v2s2ManifestDir, v2s2Archive}

var (
	saveCommand     cliconfig.SaveValues
	saveDescription = `Save an image to docker-archive or oci-archive on the local machine. Default is docker-archive.`

	_saveCommand = &cobra.Command{
		Use:   "save [flags] IMAGE",
		Short: "Save image to an archive",
		Long:  saveDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			saveCommand.InputArgs = args
			saveCommand.GlobalFlags = MainGlobalOpts
			return saveCmd(&saveCommand)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			format, err := cmd.Flags().GetString("format")
			if err != nil {
				return err
			}
			if !util.StringInSlice(format, validFormats) {
				return errors.Errorf("format value must be one of %s", strings.Join(validFormats, " "))
			}
			return nil
		},
		Example: `podman save --quiet -o myimage.tar imageID
  podman save --format docker-dir -o ubuntu-dir ubuntu
  podman save > alpine-all.tar alpine:latest`,
	}
)

func init() {
	saveCommand.Command = _saveCommand
	saveCommand.SetHelpTemplate(HelpTemplate())
	saveCommand.SetUsageTemplate(UsageTemplate())
	flags := saveCommand.Flags()
	flags.BoolVar(&saveCommand.Compress, "compress", false, "Compress tarball image layers when saving to a directory using the 'dir' transport. (default is same compression type as source)")
	flags.StringVar(&saveCommand.Format, "format", v2s2Archive, "Save image to oci-archive, oci-dir (directory with oci manifest type), docker-archive, docker-dir (directory with v2s2 manifest type)")
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
