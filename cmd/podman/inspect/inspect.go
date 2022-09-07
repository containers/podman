package inspect

import (
	"context"
	"encoding/json" // due to a bug in json-iterator it cannot be used here
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/spf13/cobra"
)

// AddInspectFlagSet takes a command and adds the inspect flags and returns an
// InspectOptions object.
func AddInspectFlagSet(cmd *cobra.Command) *entities.InspectOptions {
	opts := entities.InspectOptions{}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.Size, "size", "s", false, "Display total file size")

	formatFlagName := "format"
	flags.StringVarP(&opts.Format, formatFlagName, "f", "json", "Format the output to a Go template or json")
	_ = cmd.RegisterFlagCompletionFunc(formatFlagName, completion.AutocompleteNone)

	typeFlagName := "type"
	flags.StringVarP(&opts.Type, typeFlagName, "t", common.AllType, "Specify inspect-object type")
	_ = cmd.RegisterFlagCompletionFunc(typeFlagName, common.AutocompleteInspectType)

	validate.AddLatestFlag(cmd, &opts.Latest)
	return &opts
}

// Inspect inspects the specified container/image/pod/volume names or IDs.
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
	if options.Type == common.ImageType {
		if options.Latest {
			return nil, fmt.Errorf("latest is not supported for type %q", common.ImageType)
		}
		if options.Size {
			return nil, fmt.Errorf("size is not supported for type %q", common.ImageType)
		}
	}
	if options.Type == common.PodType && options.Size {
		return nil, fmt.Errorf("size is not supported for type %q", common.PodType)
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
	var data []interface{}
	var errs []error
	ctx := context.Background()

	if len(namesOrIDs) == 0 {
		if !i.options.Latest && !i.options.All {
			return errors.New("no names or ids specified")
		}
	}

	tmpType := i.options.Type
	if i.options.Latest {
		if len(namesOrIDs) > 0 {
			return errors.New("--latest and arguments cannot be used together")
		}
		if i.options.Type == common.AllType {
			tmpType = common.ContainerType // -l works with --type=all, defaults to containertype
		}
	}

	// Inspect - note that AllType requires us to expensively query one-by-one.
	switch tmpType {
	case common.AllType:
		allData, allErrs, err := i.inspectAll(ctx, namesOrIDs)
		if err != nil {
			return err
		}
		data = allData
		errs = allErrs
	case common.ImageType:
		imgData, allErrs, err := i.imageEngine.Inspect(ctx, namesOrIDs, i.options)
		if err != nil {
			return err
		}
		errs = allErrs
		for i := range imgData {
			data = append(data, imgData[i])
		}
	case common.ContainerType:
		ctrData, allErrs, err := i.containerEngine.ContainerInspect(ctx, namesOrIDs, i.options)
		if err != nil {
			return err
		}
		errs = allErrs
		for i := range ctrData {
			data = append(data, ctrData[i])
		}
	case common.PodType, common.PodLegacyType:
		podData, allErrs, err := i.containerEngine.PodInspect(ctx, namesOrIDs, i.options)
		if err != nil {
			return err
		}
		errs = allErrs
		for i := range podData {
			data = append(data, podData[i])
		}

	case common.NetworkType:
		networkData, allErrs, err := registry.ContainerEngine().NetworkInspect(ctx, namesOrIDs, i.options)
		if err != nil {
			return err
		}
		errs = allErrs
		for i := range networkData {
			data = append(data, networkData[i])
		}
	case common.VolumeType:
		volumeData, allErrs, err := i.containerEngine.VolumeInspect(ctx, namesOrIDs, i.options)
		if err != nil {
			return err
		}
		errs = allErrs
		for i := range volumeData {
			data = append(data, volumeData[i])
		}
	default:
		return fmt.Errorf("invalid type %q: must be %q, %q, %q, %q, %q, or %q", i.options.Type,
			common.ImageType, common.ContainerType, common.PodType, common.NetworkType, common.VolumeType, common.AllType)
	}
	// Always print an empty array
	if data == nil {
		data = []interface{}{}
	}

	var err error
	switch {
	case report.IsJSON(i.options.Format) || i.options.Format == "":
		if i.options.Type == common.PodLegacyType && len(data) == 1 {
			// We need backwards compat with the old podman pod inspect behavior.
			// https://github.com/containers/podman/pull/15675
			// TODO (5.0): consider removing this to better match other commands.
			err = printJSON(data[0])
		} else {
			err = printJSON(data)
		}
	default:
		// Landing here implies user has given a custom --format
		var rpt *report.Formatter
		format := inspectNormalize(i.options.Format, i.options.Type)
		rpt, err = report.New(os.Stdout, "inspect").Parse(report.OriginUser, format)
		if err != nil {
			return err
		}
		defer rpt.Flush()

		err = rpt.Execute(data)
	}
	if err != nil {
		errs = append(errs, fmt.Errorf("printing inspect output: %w", err))
	}

	if len(errs) > 0 {
		if len(errs) > 1 {
			for _, err := range errs[1:] {
				fmt.Fprintf(os.Stderr, "error inspecting object: %v\n", err)
			}
		}
		return fmt.Errorf("inspecting object: %w", errs[0])
	}
	return nil
}

func printJSON(data interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	// by default, json marshallers will force utf=8 from
	// a string. this breaks healthchecks that use <,>, &&.
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "     ")
	return enc.Encode(data)
}

func (i *inspector) inspectAll(ctx context.Context, namesOrIDs []string) ([]interface{}, []error, error) {
	var data []interface{}
	allErrs := []error{}
	for _, name := range namesOrIDs {
		ctrData, errs, err := i.containerEngine.ContainerInspect(ctx, []string{name}, i.options)
		if err != nil {
			return nil, nil, err
		}
		if len(errs) == 0 {
			data = append(data, ctrData[0])
			continue
		}
		imgData, errs, err := i.imageEngine.Inspect(ctx, []string{name}, i.options)
		if err != nil {
			return nil, nil, err
		}
		if len(errs) == 0 {
			data = append(data, imgData[0])
			continue
		}
		volumeData, errs, err := i.containerEngine.VolumeInspect(ctx, []string{name}, i.options)
		if err != nil {
			return nil, nil, err
		}
		if len(errs) == 0 {
			data = append(data, volumeData[0])
			continue
		}
		networkData, errs, err := registry.ContainerEngine().NetworkInspect(ctx, namesOrIDs, i.options)
		if err != nil {
			return nil, nil, err
		}
		if len(errs) == 0 {
			data = append(data, networkData[0])
			continue
		}

		podData, errs, err := i.containerEngine.PodInspect(ctx, []string{name}, i.options)
		if err != nil {
			return nil, nil, err
		}
		if len(errs) == 0 {
			data = append(data, podData[0])
			continue
		}
		if len(errs) > 0 {
			allErrs = append(allErrs, fmt.Errorf("no such object: %q", name))
			continue
		}
	}
	return data, allErrs, nil
}

func inspectNormalize(row string, inspectType string) string {
	m := regexp.MustCompile(`{{\s*\.Id\s*}}`)
	row = m.ReplaceAllString(row, "{{.ID}}")

	r := strings.NewReplacer(
		".Src", ".Source",
		".Dst", ".Destination",
		".ImageID", ".Image",
	)

	// If inspect type is `image` we need to replace
	// certain additional fields like `.Config.HealthCheck`
	// but don't want to replace them for other inspect types.
	if inspectType == common.ImageType {
		r = strings.NewReplacer(
			".Src", ".Source",
			".Dst", ".Destination",
			".ImageID", ".Image",
			".Config.Healthcheck", ".HealthCheck",
		)
	}

	return r.Replace(row)
}
