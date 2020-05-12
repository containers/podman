package images

import (
	"os"

	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	signDescription = "Create a signature file that can be used later to verify the image."
	signCommand     = &cobra.Command{
		Use:   "sign [flags] IMAGE [IMAGE...]",
		Short: "Sign an image",
		Long:  signDescription,
		RunE:  sign,
		Args:  cobra.MinimumNArgs(1),
		Example: `podman image sign --sign-by mykey imageID
  podman image sign --sign-by mykey --directory ./mykeydir imageID`,
	}
)

var (
	signOptions entities.SignOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: signCommand,
		Parent:  imageCmd,
	})
	flags := signCommand.Flags()
	flags.StringVarP(&signOptions.Directory, "directory", "d", "", "Define an alternate directory to store signatures")
	flags.StringVar(&signOptions.SignBy, "sign-by", "", "Name of the signing key")
	flags.StringVar(&signOptions.CertDir, "cert-dir", "", "`Pathname` of a directory containing TLS certificates and keys")
}

func sign(cmd *cobra.Command, args []string) error {
	if signOptions.SignBy == "" {
		return errors.Errorf("please provide an identity")
	}

	var sigStoreDir string
	if len(signOptions.Directory) > 0 {
		sigStoreDir = signOptions.Directory
		if _, err := os.Stat(sigStoreDir); err != nil {
			return errors.Wrapf(err, "invalid directory %s", sigStoreDir)
		}
	}
	_, err := registry.ImageEngine().Sign(registry.Context(), args, signOptions)
	return err
}
