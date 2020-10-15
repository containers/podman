package images

import (
	"os"

	"github.com/containers/common/pkg/auth"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// pushOptionsWrapper wraps entities.ImagepushOptions and prevents leaking
// CLI-only fields into the API types.
type pushOptionsWrapper struct {
	entities.ImagePushOptions
	TLSVerifyCLI   bool // CLI only
	CredentialsCLI string
}

var (
	pushOptions     = pushOptionsWrapper{}
	pushDescription = `Pushes a source image to a specified destination.

	The Image "DESTINATION" uses a "transport":"details" format. See podman-push(1) section "DESTINATION" for the expected format.`

	// Command: podman push
	pushCmd = &cobra.Command{
		Use:   "push [options] SOURCE [DESTINATION]",
		Short: "Push an image to a specified destination",
		Long:  pushDescription,
		RunE:  imagePush,
		Args:  cobra.RangeArgs(1, 2),
		Example: `podman push imageID docker://registry.example.com/repository:tag
		podman push imageID oci-archive:/path/to/layout:image:tag`,
	}

	// Command: podman image push
	// It's basically a clone of `pushCmd` with the exception of being a
	// child of the images command.
	imagePushCmd = &cobra.Command{
		Use:   pushCmd.Use,
		Short: pushCmd.Short,
		Long:  pushCmd.Long,
		RunE:  pushCmd.RunE,
		Args:  pushCmd.Args,
		Example: `podman image push imageID docker://registry.example.com/repository:tag
		podman image push imageID oci-archive:/path/to/layout:image:tag`,
	}
)

func init() {
	// push
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: pushCmd,
	})

	flags := pushCmd.Flags()
	pushFlags(flags)

	// images push
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: imagePushCmd,
		Parent:  imageCmd,
	})

	pushFlags(imagePushCmd.Flags())
}

// pushFlags set the flags for the push command.
func pushFlags(flags *pflag.FlagSet) {
	flags.StringVar(&pushOptions.Authfile, "authfile", auth.GetDefaultAuthFile(), "Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	flags.StringVar(&pushOptions.CertDir, "cert-dir", "", "Path to a directory containing TLS certificates and keys")
	flags.BoolVar(&pushOptions.Compress, "compress", false, "Compress tarball image layers when pushing to a directory using the 'dir' transport. (default is same compression type as source)")
	flags.StringVar(&pushOptions.CredentialsCLI, "creds", "", "`Credentials` (USERNAME:PASSWORD) to use for authenticating to a registry")
	flags.StringVar(&pushOptions.DigestFile, "digestfile", "", "Write the digest of the pushed image to the specified file")
	flags.Bool("disable-content-trust", false, "This is a Docker specific option and is a NOOP")
	flags.StringVarP(&pushOptions.Format, "format", "f", "", "Manifest type (oci, v2s1, or v2s2) to use when pushing an image using the 'dir' transport (default is manifest type of source)")
	flags.BoolVarP(&pushOptions.Quiet, "quiet", "q", false, "Suppress output information when pushing images")
	flags.BoolVar(&pushOptions.RemoveSignatures, "remove-signatures", false, "Discard any pre-existing signatures in the image")
	flags.StringVar(&pushOptions.SignaturePolicy, "signature-policy", "", "Path to a signature-policy file")
	flags.StringVar(&pushOptions.SignBy, "sign-by", "", "Add a signature at the destination using the specified key")
	flags.BoolVar(&pushOptions.TLSVerifyCLI, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries")

	if registry.IsRemote() {
		_ = flags.MarkHidden("cert-dir")
		_ = flags.MarkHidden("compress")
		_ = flags.MarkHidden("quiet")
	}
	_ = flags.MarkHidden("signature-policy")
}

// imagePush is implement the command for pushing images.
func imagePush(cmd *cobra.Command, args []string) error {
	source := args[0]
	destination := args[len(args)-1]

	// TLS verification in c/image is controlled via a `types.OptionalBool`
	// which allows for distinguishing among set-true, set-false, unspecified
	// which is important to implement a sane way of dealing with defaults of
	// boolean CLI flags.
	if cmd.Flags().Changed("tls-verify") {
		pushOptions.SkipTLSVerify = types.NewOptionalBool(!pushOptions.TLSVerifyCLI)
	}

	if pushOptions.Authfile != "" {
		if _, err := os.Stat(pushOptions.Authfile); err != nil {
			return errors.Wrapf(err, "error getting authfile %s", pushOptions.Authfile)
		}
	}

	if pushOptions.CredentialsCLI != "" {
		creds, err := util.ParseRegistryCreds(pushOptions.CredentialsCLI)
		if err != nil {
			return err
		}
		pushOptions.Username = creds.Username
		pushOptions.Password = creds.Password
	}

	// Let's do all the remaining Yoga in the API to prevent us from scattering
	// logic across (too) many parts of the code.
	return registry.ImageEngine().Push(registry.GetContext(), source, destination, pushOptions.ImagePushOptions)
}
