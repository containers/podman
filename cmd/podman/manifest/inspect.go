package manifest

import (
	"fmt"

	"github.com/containers/common/pkg/auth"
	"github.com/containers/common/pkg/completion"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	inspectOptions entities.ManifestInspectOptions
	tlsVerifyCLI   bool
	inspectCmd     = &cobra.Command{
		Use:               "inspect [options] IMAGE",
		Short:             "Display the contents of a manifest list or image index",
		Long:              "Display the contents of a manifest list or image index.",
		RunE:              inspect,
		ValidArgsFunction: common.AutocompleteImages,
		Example:           "podman manifest inspect localhost/list",
		Args:              cobra.ExactArgs(1),
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: inspectCmd,
		Parent:  manifestCmd,
	})
	flags := inspectCmd.Flags()

	authfileFlagName := "authfile"
	flags.StringVar(&inspectOptions.Authfile, authfileFlagName, auth.GetDefaultAuthFile(), "path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	_ = inspectCmd.RegisterFlagCompletionFunc(authfileFlagName, completion.AutocompleteDefault)
	flags.BoolP("verbose", "v", false, "Added for Docker compatibility")
	_ = flags.MarkHidden("verbose")
	flags.BoolVar(&tlsVerifyCLI, "tls-verify", true, "require HTTPS and verify certificates when accessing the registry")
	flags.Bool("insecure", false, "Purely for Docker compatibility")
	_ = flags.MarkHidden("insecure")
}

func inspect(cmd *cobra.Command, args []string) error {
	if cmd.Flags().Changed("authfile") {
		if err := auth.CheckAuthFile(inspectOptions.Authfile); err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("tls-verify") {
		inspectOptions.SkipTLSVerify = types.NewOptionalBool(!tlsVerifyCLI)
	} else if cmd.Flags().Changed("insecure") {
		insecure, _ := cmd.Flags().GetBool("insecure")
		inspectOptions.SkipTLSVerify = types.NewOptionalBool(insecure)
	}
	buf, err := registry.ImageEngine().ManifestInspect(registry.Context(), args[0], inspectOptions)
	if err != nil {
		return err
	}
	fmt.Println(string(buf))
	return nil
}
