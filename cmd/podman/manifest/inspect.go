package manifest

import (
	"fmt"

	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	tlsVerifyCLI bool
	inspectCmd   = &cobra.Command{
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

	flags.BoolP("verbose", "v", false, "Added for Docker compatibility")
	_ = flags.MarkHidden("verbose")
	flags.BoolVar(&tlsVerifyCLI, "tls-verify", true, "require HTTPS and verify certificates when accessing the registry")
	flags.Bool("insecure", false, "Purely for Docker compatibility")
	_ = flags.MarkHidden("insecure")
}

func inspect(cmd *cobra.Command, args []string) error {
	opts := entities.ManifestInspectOptions{}
	if cmd.Flags().Changed("tls-verify") {
		opts.SkipTLSVerify = types.NewOptionalBool(!tlsVerifyCLI)
	} else if cmd.Flags().Changed("insecure") {
		insecure, _ := cmd.Flags().GetBool("insecure")
		opts.SkipTLSVerify = types.NewOptionalBool(insecure)
	}
	buf, err := registry.ImageEngine().ManifestInspect(registry.Context(), args[0], opts)
	if err != nil {
		return err
	}
	fmt.Println(string(buf))
	return nil
}
