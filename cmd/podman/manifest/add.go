package manifest

import (
	"context"
	"fmt"

	"github.com/containers/common/pkg/auth"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/pkg/errors"
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
		Use:   "add [options] LIST LIST",
		Short: "Add images to a manifest list or image index",
		Long:  "Adds an image to a manifest list or image index.",
		RunE:  add,
		Example: `podman manifest add mylist:v1.11 image:v1.11-amd64
		podman manifest add mylist:v1.11 transport:imageName`,
		Args: cobra.ExactArgs(2),
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: addCmd,
		Parent:  manifestCmd,
	})
	flags := addCmd.Flags()
	flags.BoolVar(&manifestAddOpts.All, "all", false, "add all of the list's images if the image is a list")
	flags.StringSliceVar(&manifestAddOpts.Annotation, "annotation", nil, "set an `annotation` for the specified image")
	flags.StringVar(&manifestAddOpts.Arch, "arch", "", "override the `architecture` of the specified image")
	flags.StringVar(&manifestAddOpts.Authfile, "authfile", auth.GetDefaultAuthFile(), "path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	flags.StringVar(&manifestAddOpts.CertDir, "cert-dir", "", "use certificates at the specified path to access the registry")
	flags.StringVar(&manifestAddOpts.CredentialsCLI, "creds", "", "use `[username[:password]]` for accessing the registry")

	flags.StringSliceVar(&manifestAddOpts.Features, "features", nil, "override the `features` of the specified image")
	flags.StringVar(&manifestAddOpts.OS, "os", "", "override the `OS` of the specified image")
	flags.StringVar(&manifestAddOpts.OSVersion, "os-version", "", "override the OS `version` of the specified image")
	flags.BoolVar(&manifestAddOpts.TLSVerifyCLI, "tls-verify", true, "require HTTPS and verify certificates when accessing the registry")
	flags.StringVar(&manifestAddOpts.Variant, "variant", "", "override the `Variant` of the specified image")

	if registry.IsRemote() {
		_ = flags.MarkHidden("cert-dir")
	}
}

func add(cmd *cobra.Command, args []string) error {
	if err := auth.CheckAuthFile(manifestPushOpts.Authfile); err != nil {
		return err
	}

	manifestAddOpts.Images = []string{args[1], args[0]}

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

	listID, err := registry.ImageEngine().ManifestAdd(context.Background(), manifestAddOpts.ManifestAddOptions)
	if err != nil {
		return errors.Wrapf(err, "error adding to manifest list %s", args[0])
	}
	fmt.Printf("%s\n", listID)
	return nil
}
