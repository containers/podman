package images

import (
	"strings"

	"github.com/containers/buildah/pkg/formats"
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	inspectOpts = entities.ImageInspectOptions{}

	// Command: podman image _inspect_
	inspectCmd = &cobra.Command{
		Use:     "inspect [flags] IMAGE",
		Short:   "Display the configuration of an image",
		Long:    `Displays the low-level information on an image identified by name or ID.`,
		PreRunE: populateEngines,
		RunE:    imageInspect,
		Example: `podman image inspect alpine`,
	}

	containerEngine entities.ContainerEngine
)

// Inspect is unique in that it needs both an ImageEngine and a ContainerEngine
func populateEngines(cmd *cobra.Command, args []string) (err error) {
	// Populate registry.ImageEngine
	err = preRunE(cmd, args)
	if err != nil {
		return
	}

	// Populate registry.ContainerEngine
	containerEngine, err = registry.NewContainerEngine(cmd, args)
	return
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: inspectCmd,
		Parent:  imageCmd,
	})

	flags := inspectCmd.Flags()
	flags.BoolVarP(&inspectOpts.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.BoolVarP(&inspectOpts.Size, "size", "s", false, "Display total file size")
	flags.StringVarP(&inspectOpts.Format, "format", "f", "", "Change the output format to a Go template")

	if registry.EngineOpts.EngineMode == entities.ABIMode {
		// TODO: This is the same as V1.  We could skip creating the flag altogether in V2...
		_ = flags.MarkHidden("latest")
	}
}

const (
	inspectTypeContainer = "container"
	inspectTypeImage     = "image"
	inspectAll           = "all"
)

func imageInspect(cmd *cobra.Command, args []string) error {
	inspectType := inspectTypeImage
	latestContainer := inspectOpts.Latest

	if len(args) == 0 && !latestContainer {
		return errors.Errorf("container or image name must be specified: podman inspect [options [...]] name")
	}

	if len(args) > 0 && latestContainer {
		return errors.Errorf("you cannot provide additional arguments with --latest")
	}

	if !util.StringInSlice(inspectType, []string{inspectTypeContainer, inspectTypeImage, inspectAll}) {
		return errors.Errorf("the only recognized types are %q, %q, and %q", inspectTypeContainer, inspectTypeImage, inspectAll)
	}

	outputFormat := inspectOpts.Format
	if strings.Contains(outputFormat, "{{.Id}}") {
		outputFormat = strings.Replace(outputFormat, "{{.Id}}", formats.IDString, -1)
	}
	// These fields were renamed, so we need to provide backward compat for
	// the old names.
	if strings.Contains(outputFormat, ".Src") {
		outputFormat = strings.Replace(outputFormat, ".Src", ".Source", -1)
	}
	if strings.Contains(outputFormat, ".Dst") {
		outputFormat = strings.Replace(outputFormat, ".Dst", ".Destination", -1)
	}
	if strings.Contains(outputFormat, ".ImageID") {
		outputFormat = strings.Replace(outputFormat, ".ImageID", ".Image", -1)
	}
	_ = outputFormat
	// if latestContainer {
	// 	lc, err := ctnrRuntime.GetLatestContainer()
	// 	if err != nil {
	// 		return err
	// 	}
	// 	args = append(args, lc.ID())
	// 	inspectType = inspectTypeContainer
	// }

	// inspectedObjects, iterateErr := iterateInput(getContext(), c.Size, args, runtime, inspectType)
	// if iterateErr != nil {
	// 	return iterateErr
	// }
	//
	// var out formats.Writer
	// if outputFormat != "" && outputFormat != formats.JSONString {
	// 	// template
	// 	out = formats.StdoutTemplateArray{Output: inspectedObjects, Template: outputFormat}
	// } else {
	// 	// default is json output
	// 	out = formats.JSONStructArray{Output: inspectedObjects}
	// }
	//
	// return out.Out()
	return nil
}
