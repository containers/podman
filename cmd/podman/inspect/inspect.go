package inspect

import (
	"context"
	"fmt"
	"strings"

	"github.com/containers/buildah/pkg/formats"
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const (
	// ImageType is the image type.
	ImageType = "image"
	// ContainerType is the container type.
	ContainerType = "container"
	// AllType can be of type ImageType or ContainerType.
	AllType = "all"
)

// AddInspectFlagSet takes a command and adds the inspect flags and returns an
// InspectOptions object.
func AddInspectFlagSet(cmd *cobra.Command) *entities.InspectOptions {
	opts := entities.InspectOptions{}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.Size, "size", "s", false, "Display total file size")
	flags.StringVarP(&opts.Format, "format", "f", "json", "Format the output to a Go template or json")
	flags.StringVarP(&opts.Type, "type", "t", AllType, fmt.Sprintf("Specify inspect-oject type (%q, %q or %q)", ImageType, ContainerType, AllType))
	flags.BoolVarP(&opts.Latest, "latest", "l", false, "Act on the latest container Podman is aware of")

	return &opts
}

// Inspect inspects the specified container/image names or IDs.
func Inspect(namesOrIDs []string, options entities.InspectOptions) error {
	inspector, err := newInspector(options)
	if err != nil {
		return err
	}
	return inspector.inspect(namesOrIDs)
}

// inspector allows for inspecting images and containers.
type inspector struct {
	containerEngine entities.ContainerEngine
	imageEngine     entities.ImageEngine
	options         entities.InspectOptions
}

// newInspector creates a new inspector based on the specified options.
func newInspector(options entities.InspectOptions) (*inspector, error) {
	switch options.Type {
	case ImageType, ContainerType, AllType:
		// Valid types.
	default:
		return nil, errors.Errorf("invalid type %q: must be %q, %q or %q", options.Type, ImageType, ContainerType, AllType)
	}
	if options.Type == ImageType {
		if options.Latest {
			return nil, errors.Errorf("latest is not supported for type %q", ImageType)
		}
		if options.Size {
			return nil, errors.Errorf("size is not supported for type %q", ImageType)
		}
	}
	return &inspector{
		containerEngine: registry.ContainerEngine(),
		imageEngine:     registry.ImageEngine(),
		options:         options,
	}, nil
}

// inspect inspects the specified container/image names or IDs.
func (i *inspector) inspect(namesOrIDs []string) error {
	// data - dumping place for inspection results.
	var data []interface{} //nolint
	ctx := context.Background()

	if len(namesOrIDs) == 0 {
		if !i.options.Latest {
			return errors.New("no containers or images specified")
		}
	}

	tmpType := i.options.Type
	if i.options.Latest {
		if len(namesOrIDs) > 0 {
			return errors.New("latest and containers are not allowed")
		}
		tmpType = ContainerType // -l works with --type=all
	}

	// Inspect - note that AllType requires us to expensively query one-by-one.
	switch tmpType {
	case AllType:
		all, err := i.inspectAll(ctx, namesOrIDs)
		if err != nil {
			return err
		}
		data = all
	case ImageType:
		imgData, err := i.imageEngine.Inspect(ctx, namesOrIDs, i.options)
		if err != nil {
			return err
		}
		for i := range imgData {
			data = append(data, imgData[i])
		}
	case ContainerType:
		ctrData, err := i.containerEngine.ContainerInspect(ctx, namesOrIDs, i.options)
		if err != nil {
			return err
		}
		for i := range ctrData {
			data = append(data, ctrData[i])
		}
	default:
		return errors.Errorf("invalid type %q: must be %q, %q or %q", i.options.Type, ImageType, ContainerType, AllType)
	}

	var out formats.Writer
	if i.options.Format == "json" || i.options.Format == "" { // "" for backwards compat
		out = formats.JSONStructArray{Output: data}
	} else {
		out = formats.StdoutTemplateArray{Output: data, Template: inspectFormat(i.options.Format)}
	}
	return out.Out()
}

func (i *inspector) inspectAll(ctx context.Context, namesOrIDs []string) ([]interface{}, error) {
	var data []interface{} //nolint
	for _, name := range namesOrIDs {
		imgData, err := i.imageEngine.Inspect(ctx, []string{name}, i.options)
		if err == nil {
			data = append(data, imgData[0])
			continue
		}
		ctrData, err := i.containerEngine.ContainerInspect(ctx, []string{name}, i.options)
		if err != nil {
			return nil, err
		}
		data = append(data, ctrData[0])
	}
	return data, nil
}

func inspectFormat(row string) string {
	r := strings.NewReplacer(
		"{{.Id}}", formats.IDString,
		".Src", ".Source",
		".Dst", ".Destination",
		".ImageID", ".Image",
	)
	return r.Replace(row)
}
