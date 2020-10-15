package images

import (
	"context"
	"os"
	"strings"

	"github.com/containers/podman/v2/cmd/podman/parse"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	validFormats    = []string{define.OCIManifestDir, define.OCIArchive, define.V2s2ManifestDir, define.V2s2Archive}
	containerConfig = registry.PodmanConfig()
)

var (
	saveDescription = `Save an image to docker-archive or oci-archive on the local machine. Default is docker-archive.`

	saveCommand = &cobra.Command{
		Use:   "save [options] IMAGE [IMAGE...]",
		Short: "Save image(s) to an archive",
		Long:  saveDescription,
		RunE:  save,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.Errorf("need at least 1 argument")
			}
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
	imageSaveCommand = &cobra.Command{
		Args:  saveCommand.Args,
		Use:   saveCommand.Use,
		Short: saveCommand.Short,
		Long:  saveCommand.Long,
		RunE:  saveCommand.RunE,
		Example: `podman image save --quiet -o myimage.tar imageID
  podman image save --format docker-dir -o ubuntu-dir ubuntu
  podman image save > alpine-all.tar alpine:latest`,
	}
)

var (
	saveOpts entities.ImageSaveOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: saveCommand,
	})
	saveFlags(saveCommand.Flags())

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: imageSaveCommand,
		Parent:  imageCmd,
	})
	saveFlags(imageSaveCommand.Flags())
}

func saveFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&saveOpts.Compress, "compress", false, "Compress tarball image layers when saving to a directory using the 'dir' transport. (default is same compression type as source)")
	flags.StringVar(&saveOpts.Format, "format", define.V2s2Archive, "Save image to oci-archive, oci-dir (directory with oci manifest type), docker-archive, docker-dir (directory with v2s2 manifest type)")
	flags.StringVarP(&saveOpts.Output, "output", "o", "", "Write to a specified file (default: stdout, which must be redirected)")
	flags.BoolVarP(&saveOpts.Quiet, "quiet", "q", false, "Suppress the output")
	flags.BoolVarP(&saveOpts.MultiImageArchive, "multi-image-archive", "m", containerConfig.Engine.MultiImageArchive, "Interpret additional arguments as images not tags and create a multi-image-archive (only for docker-archive)")
}

func save(cmd *cobra.Command, args []string) (finalErr error) {
	var (
		tags      []string
		succeeded = false
	)
	if cmd.Flag("compress").Changed && (saveOpts.Format != define.OCIManifestDir && saveOpts.Format != define.V2s2ManifestDir && saveOpts.Format == "") {
		return errors.Errorf("--compress can only be set when --format is either 'oci-dir' or 'docker-dir'")
	}
	if len(saveOpts.Output) == 0 {
		saveOpts.Quiet = true
		fi := os.Stdout
		if terminal.IsTerminal(int(fi.Fd())) {
			return errors.Errorf("refusing to save to terminal. Use -o flag or redirect")
		}
		pipePath, cleanup, err := setupPipe()
		if err != nil {
			return err
		}
		if cleanup != nil {
			defer func() {
				errc := cleanup()
				if succeeded {
					writeErr := <-errc
					if writeErr != nil && finalErr == nil {
						finalErr = writeErr
					}
				}
			}()
		}
		saveOpts.Output = pipePath
	}
	if err := parse.ValidateFileName(saveOpts.Output); err != nil {
		return err
	}
	if len(args) > 1 {
		tags = args[1:]
	}

	err := registry.ImageEngine().Save(context.Background(), args[0], tags, saveOpts)
	if err == nil {
		succeeded = true
	}
	return err
}
