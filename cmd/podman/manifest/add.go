package manifest

import (
	"context"
	"fmt"

	"github.com/containers/common/pkg/auth"
	"github.com/containers/common/pkg/completion"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/spf13/cobra"
)

// manifestAddOptsWrapper wraps entities.ManifestAddOptions and prevents leaking
// CLI-only fields into the API types.
type manifestAddOptsWrapper struct {
	entities.ManifestAddOptions

	TLSVerifyCLI   bool // CLI only
	CredentialsCLI string
}

var (
	manifestAddOpts = manifestAddOptsWrapper{}
	addCmd          = &cobra.Command{
		Use:               "add [options] LIST IMAGE [IMAGE...]",
		Short:             "Add images to a manifest list or image index",
		Long:              "Adds an image to a manifest list or image index.",
		RunE:              add,
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: common.AutocompleteImages,
		Example: `podman manifest add mylist:v1.11 image:v1.11-amd64
		podman manifest add mylist:v1.11 transport:imageName`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: addCmd,
		Parent:  manifestCmd,
	})
	flags := addCmd.Flags()
	flags.BoolVar(&manifestAddOpts.All, "all", false, "add all of the list's images if the image is a list")

	annotationFlagName := "annotation"
	flags.StringSliceVar(&manifestAddOpts.Annotation, annotationFlagName, nil, "set an `annotation` for the specified image")
	_ = addCmd.RegisterFlagCompletionFunc(annotationFlagName, completion.AutocompleteNone)

	archFlagName := "arch"
	flags.StringVar(&manifestAddOpts.Arch, archFlagName, "", "override the `architecture` of the specified image")
	_ = addCmd.RegisterFlagCompletionFunc(archFlagName, completion.AutocompleteArch)

	authfileFlagName := "authfile"
	flags.StringVar(&manifestAddOpts.Authfile, authfileFlagName, auth.GetDefaultAuthFile(), "path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	_ = addCmd.RegisterFlagCompletionFunc(authfileFlagName, completion.AutocompleteDefault)

	certDirFlagName := "cert-dir"
	flags.StringVar(&manifestAddOpts.CertDir, certDirFlagName, "", "use certificates at the specified path to access the registry")
	_ = addCmd.RegisterFlagCompletionFunc(certDirFlagName, completion.AutocompleteDefault)

	credsFlagName := "creds"
	flags.StringVar(&manifestAddOpts.CredentialsCLI, credsFlagName, "", "use `[username[:password]]` for accessing the registry")
	_ = addCmd.RegisterFlagCompletionFunc(credsFlagName, completion.AutocompleteNone)

	featuresFlagName := "features"
	flags.StringSliceVar(&manifestAddOpts.Features, featuresFlagName, nil, "override the `features` of the specified image")
	_ = addCmd.RegisterFlagCompletionFunc(featuresFlagName, completion.AutocompleteNone)

	osFlagName := "os"
	flags.StringVar(&manifestAddOpts.OS, osFlagName, "", "override the `OS` of the specified image")
	_ = addCmd.RegisterFlagCompletionFunc(osFlagName, completion.AutocompleteOS)

	osVersionFlagName := "os-version"
	flags.StringVar(&manifestAddOpts.OSVersion, osVersionFlagName, "", "override the OS `version` of the specified image")
	_ = addCmd.RegisterFlagCompletionFunc(osVersionFlagName, completion.AutocompleteNone)

	flags.BoolVar(&manifestAddOpts.TLSVerifyCLI, "tls-verify", true, "require HTTPS and verify certificates when accessing the registry")

	variantFlagName := "variant"
	flags.StringVar(&manifestAddOpts.Variant, variantFlagName, "", "override the `Variant` of the specified image")
	_ = addCmd.RegisterFlagCompletionFunc(variantFlagName, completion.AutocompleteNone)

	if registry.IsRemote() {
		_ = flags.MarkHidden("cert-dir")
	}
}

func add(cmd *cobra.Command, args []string) error {
	if err := auth.CheckAuthFile(manifestPushOpts.Authfile); err != nil {
		return err
	}

	if manifestAddOpts.CredentialsCLI != "" {
		creds, err := util.ParseRegistryCreds(manifestAddOpts.CredentialsCLI)
		if err != nil {
			return err
		}
		manifestAddOpts.Username = creds.Username
		manifestAddOpts.Password = creds.Password
	}

	// TLS verification in c/image is controlled via a `types.OptionalBool`
	// which allows for distinguishing among set-true, set-false, unspecified
	// which is important to implement a sane way of dealing with defaults of
	// boolean CLI flags.
	if cmd.Flags().Changed("tls-verify") {
		manifestAddOpts.SkipTLSVerify = types.NewOptionalBool(!manifestAddOpts.TLSVerifyCLI)
	}

	listID, err := registry.ImageEngine().ManifestAdd(context.Background(), args[0], args[1:], manifestAddOpts.ManifestAddOptions)
	if err != nil {
		return err
	}
	fmt.Println(listID)
	return nil
}
