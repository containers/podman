package manifest

import (
	"github.com/containers/common/pkg/auth"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// manifestPushOptsWrapper wraps entities.ManifestPushOptions and prevents leaking
// CLI-only fields into the API types.
type manifestPushOptsWrapper struct {
	entities.ManifestPushOptions

	TLSVerifyCLI   bool // CLI only
	CredentialsCLI string
}

var (
	manifestPushOpts = manifestPushOptsWrapper{}
	pushCmd          = &cobra.Command{
		Use:     "push [flags] SOURCE DESTINATION",
		Short:   "Push a manifest list or image index to a registry",
		Long:    "Pushes manifest lists and image indexes to registries.",
		RunE:    push,
		Example: `podman manifest push mylist:v1.11 quay.io/myimagelist`,
		Args:    cobra.ExactArgs(2),
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: pushCmd,
		Parent:  manifestCmd,
	})
	flags := pushCmd.Flags()
	flags.BoolVar(&manifestPushOpts.Purge, "purge", false, "remove the manifest list if push succeeds")
	flags.BoolVar(&manifestPushOpts.All, "all", false, "also push the images in the list")
	flags.StringVar(&manifestPushOpts.Authfile, "authfile", auth.GetDefaultAuthFile(), "path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	flags.StringVar(&manifestPushOpts.CertDir, "cert-dir", "", "use certificates at the specified path to access the registry")
	flags.StringVar(&manifestPushOpts.CredentialsCLI, "creds", "", "use `[username[:password]]` for accessing the registry")
	flags.StringVar(&manifestPushOpts.DigestFile, "digestfile", "", "after copying the image, write the digest of the resulting digest to the file")
	flags.StringVarP(&manifestPushOpts.Format, "format", "f", "", "manifest type (oci or v2s2) to attempt to use when pushing the manifest list (default is manifest type of source)")
	flags.BoolVarP(&manifestPushOpts.RemoveSignatures, "remove-signatures", "", false, "don't copy signatures when pushing images")
	flags.StringVar(&manifestPushOpts.SignBy, "sign-by", "", "sign the image using a GPG key with the specified `FINGERPRINT`")
	flags.BoolVar(&manifestPushOpts.TLSVerifyCLI, "tls-verify", true, "require HTTPS and verify certificates when accessing the registry")
	flags.BoolVarP(&manifestPushOpts.Quiet, "quiet", "q", false, "don't output progress information when pushing lists")
	if registry.IsRemote() {
		_ = flags.MarkHidden("authfile")
		_ = flags.MarkHidden("cert-dir")
		_ = flags.MarkHidden("tls-verify")
	}
}

func push(cmd *cobra.Command, args []string) error {
	if err := auth.CheckAuthFile(manifestPushOpts.Authfile); err != nil {
		return err
	}
	listImageSpec := args[0]
	destSpec := args[1]
	if listImageSpec == "" {
		return errors.Errorf(`invalid image name "%s"`, listImageSpec)
	}
	if destSpec == "" {
		return errors.Errorf(`invalid destination "%s"`, destSpec)
	}

	if manifestPushOpts.CredentialsCLI != "" {
		creds, err := util.ParseRegistryCreds(manifestPushOpts.CredentialsCLI)
		if err != nil {
			return err
		}
		manifestPushOpts.Username = creds.Username
		manifestPushOpts.Password = creds.Password
	}

	// TLS verification in c/image is controlled via a `types.OptionalBool`
	// which allows for distinguishing among set-true, set-false, unspecified
	// which is important to implement a sane way of dealing with defaults of
	// boolean CLI flags.
	if cmd.Flags().Changed("tls-verify") {
		manifestPushOpts.SkipTLSVerify = types.NewOptionalBool(!manifestPushOpts.TLSVerifyCLI)
	}
	if err := registry.ImageEngine().ManifestPush(registry.Context(), args, manifestPushOpts.ManifestPushOptions); err != nil {
		return errors.Wrapf(err, "error pushing manifest %s to %s", listImageSpec, destSpec)
	}
	return nil
}
