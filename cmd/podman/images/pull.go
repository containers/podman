package images

import (
	"fmt"
	"os"

	"github.com/containers/common/pkg/auth"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// pullOptionsWrapper wraps entities.ImagePullOptions and prevents leaking
// CLI-only fields into the API types.
type pullOptionsWrapper struct {
	entities.ImagePullOptions
	TLSVerifyCLI bool // CLI only
}

var (
	pullOptions     = pullOptionsWrapper{}
	pullDescription = `Pulls an image from a registry and stores it locally.

  An image can be pulled by tag or digest. If a tag is not specified, the image with the 'latest' tag is pulled.`

	// Command: podman pull
	pullCmd = &cobra.Command{
		Use:   "pull [flags] IMAGE",
		Args:  cobra.ExactArgs(1),
		Short: "Pull an image from a registry",
		Long:  pullDescription,
		RunE:  imagePull,
		Example: `podman pull imageName
  podman pull fedora:latest`,
	}

	// Command: podman image pull
	// It's basically a clone of `pullCmd` with the exception of being a
	// child of the images command.
	imagesPullCmd = &cobra.Command{
		Use:   pullCmd.Use,
		Short: pullCmd.Short,
		Long:  pullCmd.Long,
		RunE:  pullCmd.RunE,
		Args:  cobra.ExactArgs(1),
		Example: `podman image pull imageName
  podman image pull fedora:latest`,
	}
)

func init() {
	// pull
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: pullCmd,
	})

	flags := pullCmd.Flags()
	pullFlags(flags)

	// images pull
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: imagesPullCmd,
		Parent:  imageCmd,
	})

	imagesPullFlags := imagesPullCmd.Flags()
	pullFlags(imagesPullFlags)
}

// pullFlags set the flags for the pull command.
func pullFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&pullOptions.AllTags, "all-tags", false, "All tagged images in the repository will be pulled")
	flags.StringVar(&pullOptions.Authfile, "authfile", auth.GetDefaultAuthFile(), "Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	flags.StringVar(&pullOptions.CertDir, "cert-dir", "", "`Pathname` of a directory containing TLS certificates and keys")
	flags.StringVar(&pullOptions.Credentials, "creds", "", "`Credentials` (USERNAME:PASSWORD) to use for authenticating to a registry")
	flags.StringVar(&pullOptions.OverrideArch, "override-arch", "", "Use `ARCH` instead of the architecture of the machine for choosing images")
	flags.StringVar(&pullOptions.OverrideOS, "override-os", "", "Use `OS` instead of the running OS for choosing images")
	flags.BoolVarP(&pullOptions.Quiet, "quiet", "q", false, "Suppress output information when pulling images")
	flags.StringVar(&pullOptions.SignaturePolicy, "signature-policy", "", "`Pathname` of signature policy file (not usually used)")
	flags.BoolVar(&pullOptions.TLSVerifyCLI, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries")

	if registry.IsRemote() {
		_ = flags.MarkHidden("authfile")
		_ = flags.MarkHidden("cert-dir")
		_ = flags.MarkHidden("tls-verify")
	}
	_ = flags.MarkHidden("signature-policy")
}

// imagePull is implement the command for pulling images.
func imagePull(cmd *cobra.Command, args []string) error {
	// TLS verification in c/image is controlled via a `types.OptionalBool`
	// which allows for distinguishing among set-true, set-false, unspecified
	// which is important to implement a sane way of dealing with defaults of
	// boolean CLI flags.
	if cmd.Flags().Changed("tls-verify") {
		pullOptions.SkipTLSVerify = types.NewOptionalBool(!pullOptions.TLSVerifyCLI)
	}
	if pullOptions.Authfile != "" {
		if _, err := os.Stat(pullOptions.Authfile); err != nil {
			return errors.Wrapf(err, "error getting authfile %s", pullOptions.Authfile)
		}
	}

	// Let's do all the remaining Yoga in the API to prevent us from
	// scattering logic across (too) many parts of the code.
	pullReport, err := registry.ImageEngine().Pull(registry.GetContext(), args[0], pullOptions.ImagePullOptions)
	if err != nil {
		return err
	}

	if len(pullReport.Images) > 1 {
		fmt.Println("Pulled Images:")
	}
	for _, img := range pullReport.Images {
		fmt.Println(img)
	}

	return nil
}
