package manifest

import (
	"errors"
	"fmt"

	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/spf13/cobra"
)

// manifestCreateOptsWrapper wraps entities.ManifestCreateOptions and prevents leaking
// CLI-only fields into the API types.
type manifestCreateOptsWrapper struct {
	entities.ManifestCreateOptions

	TLSVerifyCLI, Insecure bool // CLI only
}

var (
	manifestCreateOpts = manifestCreateOptsWrapper{}
	createCmd          = &cobra.Command{
		Use:               "create [options] LIST [IMAGE...]",
		Short:             "Create manifest list or image index",
		Long:              "Creates manifest lists or image indexes.",
		RunE:              create,
		ValidArgsFunction: common.AutocompleteImages,
		Example: `podman manifest create mylist:v1.11
  podman manifest create mylist:v1.11 arch-specific-image-to-add
  podman manifest create mylist:v1.11 arch-specific-image-to-add another-arch-specific-image-to-add
  podman manifest create --all mylist:v1.11 transport:tagged-image-to-add`,
		Args: cobra.MinimumNArgs(1),
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: createCmd,
		Parent:  manifestCmd,
	})
	flags := createCmd.Flags()
	flags.BoolVar(&manifestCreateOpts.All, "all", false, "add all of the lists' images if the images to add are lists")
	flags.BoolVarP(&manifestCreateOpts.Amend, "amend", "a", false, "modify an existing list if one with the desired name already exists")
	flags.BoolVar(&manifestCreateOpts.Insecure, "insecure", false, "neither require HTTPS nor verify certificates when accessing the registry")
	_ = flags.MarkHidden("insecure")
	flags.BoolVar(&manifestCreateOpts.TLSVerifyCLI, "tls-verify", true, "require HTTPS and verify certificates when accessing the registry")
}

func create(cmd *cobra.Command, args []string) error {
	// TLS verification in c/image is controlled via a `types.OptionalBool`
	// which allows for distinguishing among set-true, set-false, unspecified
	// which is important to implement a sane way of dealing with defaults of
	// boolean CLI flags.
	if cmd.Flags().Changed("tls-verify") {
		manifestCreateOpts.SkipTLSVerify = types.NewOptionalBool(!manifestCreateOpts.TLSVerifyCLI)
	}
	if cmd.Flags().Changed("insecure") {
		if manifestCreateOpts.SkipTLSVerify != types.OptionalBoolUndefined {
			return errors.New("--insecure may not be used with --tls-verify")
		}
		manifestCreateOpts.SkipTLSVerify = types.NewOptionalBool(manifestCreateOpts.Insecure)
	}

	imageID, err := registry.ImageEngine().ManifestCreate(registry.Context(), args[0], args[1:], manifestCreateOpts.ManifestCreateOptions)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", imageID)
	return nil
}
