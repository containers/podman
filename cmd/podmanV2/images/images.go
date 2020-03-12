package images

import (
	"strings"

	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// podman _images_
	imagesCmd = &cobra.Command{
		Use:               strings.Replace(listCmd.Use, "list", "images", 1),
		Short:             listCmd.Short,
		Long:              listCmd.Long,
		PersistentPreRunE: preRunE,
		RunE:              images,
		Example:           strings.Replace(listCmd.Example, "podman image list", "podman images", -1),
	}

	imagesOpts = entities.ImageListOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: imagesCmd,
	})
	imagesCmd.SetHelpTemplate(registry.HelpTemplate())
	imagesCmd.SetUsageTemplate(registry.UsageTemplate())

	flags := imagesCmd.Flags()
	flags.BoolVarP(&imagesOpts.All, "all", "a", false, "Show all images (default hides intermediate images)")
	flags.BoolVar(&imagesOpts.Digests, "digests", false, "Show digests")
	flags.StringSliceVarP(&imagesOpts.Filter, "filter", "f", []string{}, "Filter output based on conditions provided (default [])")
	flags.StringVar(&imagesOpts.Format, "format", "", "Change the output format to JSON or a Go template")
	flags.BoolVarP(&imagesOpts.Noheading, "noheading", "n", false, "Do not print column headings")
	// TODO Need to learn how to deal with second name being a string instead of a char.
	// This needs to be "no-trunc, notruncate"
	flags.BoolVar(&imagesOpts.NoTrunc, "no-trunc", false, "Do not truncate output")
	flags.BoolVar(&imagesOpts.NoTrunc, "notruncate", false, "Do not truncate output")
	flags.BoolVarP(&imagesOpts.Quiet, "quiet", "q", false, "Display only image IDs")
	flags.StringVar(&imagesOpts.Sort, "sort", "created", "Sort by created, id, repository, size, or tag")
	flags.BoolVarP(&imagesOpts.History, "history", "", false, "Display the image name history")
}
