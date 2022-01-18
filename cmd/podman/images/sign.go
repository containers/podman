package images

import (
	"os"

	"github.com/containers/common/pkg/auth"
	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	signDescription = "Create a signature file that can be used later to verify the image."
	signCommand     = &cobra.Command{
		Annotations:       map[string]string{registry.EngineMode: registry.ABIMode},
		Use:               "sign [options] IMAGE [IMAGE...]",
		Short:             "Sign an image",
		Long:              signDescription,
		RunE:              sign,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: common.AutocompleteImages,
		Example: `podman image sign --sign-by mykey imageID
  podman image sign --sign-by mykey --directory ./mykeydir imageID`,
	}
)

var (
	signOptions entities.SignOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: signCommand,
		Parent:  imageCmd,
	})
	flags := signCommand.Flags()
	directoryFlagName := "directory"
	flags.StringVarP(&signOptions.Directory, directoryFlagName, "d", "", "Define an alternate directory to store signatures")
	_ = signCommand.RegisterFlagCompletionFunc(directoryFlagName, completion.AutocompleteDefault)

	signByFlagName := "sign-by"
	flags.StringVar(&signOptions.SignBy, signByFlagName, "", "Name of the signing key")
	_ = signCommand.RegisterFlagCompletionFunc(signByFlagName, completion.AutocompleteNone)

	certDirFlagName := "cert-dir"
	flags.StringVar(&signOptions.CertDir, certDirFlagName, "", "`Pathname` of a directory containing TLS certificates and keys")
	_ = signCommand.RegisterFlagCompletionFunc(certDirFlagName, completion.AutocompleteDefault)
	flags.BoolVarP(&signOptions.All, "all", "a", false, "Sign all the manifests of the multi-architecture image")

	authfileFlagName := "authfile"
	flags.StringVar(&signOptions.Authfile, authfileFlagName, auth.GetDefaultAuthFile(), "Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	_ = signCommand.RegisterFlagCompletionFunc(authfileFlagName, completion.AutocompleteDefault)
}

func sign(cmd *cobra.Command, args []string) error {
	if signOptions.SignBy == "" {
		return errors.Errorf("please provide an identity")
	}

	var sigStoreDir string
	if len(signOptions.Directory) > 0 {
		sigStoreDir = signOptions.Directory
		if _, err := os.Stat(sigStoreDir); err != nil {
			return err
		}
	}
	_, err := registry.ImageEngine().Sign(registry.Context(), args, signOptions)
	return err
}
