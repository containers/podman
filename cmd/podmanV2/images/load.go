package images

import (
	"context"
	"fmt"

	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	loadDescription = "Loads an image from a locally stored archive (tar file) into container storage."
	loadCommand     = &cobra.Command{
		Use:               "load [flags] [NAME[:TAG]]",
		Short:             "Load an image from container archive",
		Long:              loadDescription,
		RunE:              load,
		Args:              cobra.MaximumNArgs(1),
		PersistentPreRunE: preRunE,
	}
)

var (
	loadOpts entities.ImageLoadOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: loadCommand,
	})

	loadCommand.SetHelpTemplate(registry.HelpTemplate())
	loadCommand.SetUsageTemplate(registry.UsageTemplate())
	flags := loadCommand.Flags()
	flags.StringVarP(&loadOpts.Input, "input", "i", "", "Read from specified archive file (default: stdin)")
	flags.BoolVarP(&loadOpts.Quiet, "quiet", "q", false, "Suppress the output")
	flags.StringVar(&loadOpts.SignaturePolicy, "signature-policy", "", "Pathname of signature policy file")
	if registry.IsRemote() {
		_ = flags.MarkHidden("signature-policy")
	}

}

func load(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		repo, err := image.NormalizedTag(args[0])
		if err != nil {
			return err
		}
		loadOpts.Name = repo.Name()
	}
	response, err := registry.ImageEngine().Load(context.Background(), loadOpts)
	if err != nil {
		return err
	}
	fmt.Println("Loaded image: " + response.Name)
	return nil
}
