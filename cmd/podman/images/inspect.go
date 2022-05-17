package images

import (
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/inspect"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/domain/entities"
	inspectTypes "github.com/containers/podman/v4/pkg/inspect"
	"github.com/spf13/cobra"
)

var (
	// Command: podman image _inspect_
	inspectCmd = &cobra.Command{
		Use:               "inspect [options] IMAGE [IMAGE...]",
		Short:             "Display the configuration of an image",
		Long:              `Displays the low-level information of an image identified by name or ID.`,
		RunE:              inspectExec,
		ValidArgsFunction: common.AutocompleteImages,
		Example: `podman image inspect alpine
  podman image inspect --format "imageId: {{.Id}} size: {{.Size}}" alpine
  podman image inspect --format "image: {{.ImageName}} driver: {{.Driver}}" myctr`,
	}
	inspectOpts *entities.InspectOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: inspectCmd,
		Parent:  imageCmd,
	})
	inspectOpts = new(entities.InspectOptions)
	flags := inspectCmd.Flags()

	formatFlagName := "format"
	flags.StringVarP(&inspectOpts.Format, formatFlagName, "f", "json", "Format the output to a Go template or json")
	_ = inspectCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&inspectTypes.ImageData{}))
}

func inspectExec(cmd *cobra.Command, args []string) error {
	inspectOpts.Type = common.ImageType
	return inspect.Inspect(args, *inspectOpts)
}
