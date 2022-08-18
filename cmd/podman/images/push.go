package images

import (
	"fmt"
	"os"

	"github.com/containers/common/pkg/auth"
	"github.com/containers/common/pkg/completion"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/spf13/cobra"
)

// pushOptionsWrapper wraps entities.ImagepushOptions and prevents leaking
// CLI-only fields into the API types.
type pushOptionsWrapper struct {
	entities.ImagePushOptions
	TLSVerifyCLI          bool // CLI only
	CredentialsCLI        string
	SignPassphraseFileCLI string
	EncryptionKeys        []string
	EncryptLayers         []int
}

var (
	pushOptions     = pushOptionsWrapper{}
	pushDescription = `Pushes a source image to a specified destination.

	The Image "DESTINATION" uses a "transport":"details" format. See podman-push(1) section "DESTINATION" for the expected format.`

	// Command: podman push
	pushCmd = &cobra.Command{
		Use:               "push [options] IMAGE [DESTINATION]",
		Short:             "Push an image to a specified destination",
		Long:              pushDescription,
		RunE:              imagePush,
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: common.AutocompleteImages,
		Example: `podman push imageID docker://registry.example.com/repository:tag
		podman push imageID oci-archive:/path/to/layout:image:tag`,
	}

	// Command: podman image push
	// It's basically a clone of `pushCmd` with the exception of being a
	// child of the images command.
	imagePushCmd = &cobra.Command{
		Use:               pushCmd.Use,
		Short:             pushCmd.Short,
		Long:              pushCmd.Long,
		RunE:              pushCmd.RunE,
		Args:              pushCmd.Args,
		ValidArgsFunction: pushCmd.ValidArgsFunction,
		Example: `podman image push imageID docker://registry.example.com/repository:tag
		podman image push imageID oci-archive:/path/to/layout:image:tag`,
	}
)

func init() {
	// push
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: pushCmd,
	})
	pushFlags(pushCmd)

	// images push
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: imagePushCmd,
		Parent:  imageCmd,
	})
	pushFlags(imagePushCmd)
}

// pushFlags set the flags for the push command.
func pushFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	// For now default All flag to true, for pushing of manifest lists
	pushOptions.All = true
	authfileFlagName := "authfile"
	flags.StringVar(&pushOptions.Authfile, authfileFlagName, auth.GetDefaultAuthFile(), "Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	_ = cmd.RegisterFlagCompletionFunc(authfileFlagName, completion.AutocompleteDefault)

	certDirFlagName := "cert-dir"
	flags.StringVar(&pushOptions.CertDir, certDirFlagName, "", "Path to a directory containing TLS certificates and keys")
	_ = cmd.RegisterFlagCompletionFunc(certDirFlagName, completion.AutocompleteDefault)

	flags.BoolVar(&pushOptions.Compress, "compress", false, "Compress tarball image layers when pushing to a directory using the 'dir' transport. (default is same compression type as source)")

	credsFlagName := "creds"
	flags.StringVar(&pushOptions.CredentialsCLI, credsFlagName, "", "`Credentials` (USERNAME:PASSWORD) to use for authenticating to a registry")
	_ = cmd.RegisterFlagCompletionFunc(credsFlagName, completion.AutocompleteNone)

	flags.Bool("disable-content-trust", false, "This is a Docker specific option and is a NOOP")

	digestfileFlagName := "digestfile"
	flags.StringVar(&pushOptions.DigestFile, digestfileFlagName, "", "Write the digest of the pushed image to the specified file")
	_ = cmd.RegisterFlagCompletionFunc(digestfileFlagName, completion.AutocompleteDefault)

	formatFlagName := "format"
	flags.StringVarP(&pushOptions.Format, formatFlagName, "f", "", "Manifest type (oci, v2s2, or v2s1) to use in the destination (default is manifest type of source, with fallbacks)")
	_ = cmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteManifestFormat)

	flags.BoolVarP(&pushOptions.Quiet, "quiet", "q", false, "Suppress output information when pushing images")
	flags.BoolVar(&pushOptions.RemoveSignatures, "remove-signatures", false, "Discard any pre-existing signatures in the image")

	signByFlagName := "sign-by"
	flags.StringVar(&pushOptions.SignBy, signByFlagName, "", "Add a signature at the destination using the specified key")
	_ = cmd.RegisterFlagCompletionFunc(signByFlagName, completion.AutocompleteNone)

	signBySigstorePrivateKeyFlagName := "sign-by-sigstore-private-key"
	flags.StringVar(&pushOptions.SignBySigstorePrivateKeyFile, signBySigstorePrivateKeyFlagName, "", "Sign the image using a sigstore private key at `PATH`")
	_ = cmd.RegisterFlagCompletionFunc(signBySigstorePrivateKeyFlagName, completion.AutocompleteDefault)

	signPassphraseFileFlagName := "sign-passphrase-file"
	flags.StringVar(&pushOptions.SignPassphraseFileCLI, signPassphraseFileFlagName, "", "Read a passphrase for signing an image from `PATH`")
	_ = cmd.RegisterFlagCompletionFunc(signPassphraseFileFlagName, completion.AutocompleteDefault)

	flags.BoolVar(&pushOptions.TLSVerifyCLI, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries")

	compressionFormat := "compression-format"
	flags.StringVar(&pushOptions.CompressionFormat, compressionFormat, "", "compression format to use")
	_ = cmd.RegisterFlagCompletionFunc(compressionFormat, common.AutocompleteCompressionFormat)

	encryptionKeysFlagName := "encryption-key"
	flags.StringSliceVar(&pushOptions.EncryptionKeys, encryptionKeysFlagName, nil, "Key with the encryption protocol to use to encrypt the image (e.g. jwe:/path/to/key.pem)")
	_ = cmd.RegisterFlagCompletionFunc(encryptionKeysFlagName, completion.AutocompleteDefault)

	encryptLayersFlagName := "encrypt-layer"
	flags.IntSliceVar(&pushOptions.EncryptLayers, encryptLayersFlagName, nil, "Layers to encrypt, 0-indexed layer indices with support for negative indexing (e.g. 0 is the first layer, -1 is the last layer). If not defined, will encrypt all layers if encryption-key flag is specified")
	_ = cmd.RegisterFlagCompletionFunc(encryptLayersFlagName, completion.AutocompleteDefault)

	if registry.IsRemote() {
		_ = flags.MarkHidden("cert-dir")
		_ = flags.MarkHidden("compress")
		_ = flags.MarkHidden("digestfile")
		_ = flags.MarkHidden("quiet")
		_ = flags.MarkHidden(signByFlagName)
		_ = flags.MarkHidden(signBySigstorePrivateKeyFlagName)
		_ = flags.MarkHidden(signPassphraseFileFlagName)
		_ = flags.MarkHidden(encryptionKeysFlagName)
		_ = flags.MarkHidden(encryptLayersFlagName)
	}
	if !registry.IsRemote() {
		flags.StringVar(&pushOptions.SignaturePolicy, "signature-policy", "", "Path to a signature-policy file")
		_ = flags.MarkHidden("signature-policy")
	}
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
			return err
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

	if !pushOptions.Quiet {
		pushOptions.Writer = os.Stderr
	}

	if err := common.PrepareSigningPassphrase(&pushOptions.ImagePushOptions, pushOptions.SignPassphraseFileCLI); err != nil {
		return err
	}

	encConfig, encLayers, err := util.EncryptConfig(pushOptions.EncryptionKeys, pushOptions.EncryptLayers)
	if err != nil {
		return fmt.Errorf("unable to obtain encryption config: %w", err)
	}
	pushOptions.OciEncryptConfig = encConfig
	pushOptions.OciEncryptLayers = encLayers

	// Let's do all the remaining Yoga in the API to prevent us from scattering
	// logic across (too) many parts of the code.
	return registry.ImageEngine().Push(registry.GetContext(), source, destination, pushOptions.ImagePushOptions)
}
