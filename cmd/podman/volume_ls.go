package main

import (
	"reflect"
	"strings"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/formats"
	"github.com/containers/libpod/libpod/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// volumeOptions is the "ls" command options
type volumeLsOptions struct {
	Format string
	Quiet  bool
}

// volumeLsTemplateParams is the template parameters to list the volumes
type volumeLsTemplateParams struct {
	Name       string
	Labels     string
	MountPoint string
	Driver     string
	Options    string
	Scope      string
}

// volumeLsJSONParams is the JSON parameters to list the volumes
type volumeLsJSONParams struct {
	Name       string            `json:"name"`
	Labels     map[string]string `json:"labels"`
	MountPoint string            `json:"mountPoint"`
	Driver     string            `json:"driver"`
	Options    map[string]string `json:"options"`
	Scope      string            `json:"scope"`
}

var (
	volumeLsCommand cliconfig.VolumeLsValues

	volumeLsDescription = `
podman volume ls

List all available volumes. The output of the volumes can be filtered
and the output format can be changed to JSON or a user specified Go template.
`
	_volumeLsCommand = &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List volumes",
		Long:    volumeLsDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			volumeLsCommand.InputArgs = args
			volumeLsCommand.GlobalFlags = MainGlobalOpts
			return volumeLsCmd(&volumeLsCommand)
		},
	}
)

func init() {
	volumeLsCommand.Command = _volumeLsCommand
	volumeLsCommand.SetUsageTemplate(UsageTemplate())
	flags := volumeLsCommand.Flags()

	flags.StringVarP(&volumeLsCommand.Filter, "filter", "f", "", "Filter volume output")
	flags.StringVar(&volumeLsCommand.Format, "format", "table {{.Driver}}\t{{.Name}}", "Format volume output using Go template")
	flags.BoolVarP(&volumeLsCommand.Quiet, "quiet", "q", false, "Print volume output in quiet mode")
}

func volumeLsCmd(c *cliconfig.VolumeLsValues) error {
	runtime, err := adapter.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	if len(c.InputArgs) > 0 {
		return errors.Errorf("too many arguments, ls takes no arguments")
	}

	opts := volumeLsOptions{
		Quiet: c.Quiet,
	}
	opts.Format = genVolLsFormat(c)

	// Get the filter functions based on any filters set
	var filterFuncs []adapter.VolumeFilter
	if c.Filter != "" {
		filters := strings.Split(c.Filter, ",")
		for _, f := range filters {
			filterSplit := strings.Split(f, "=")
			if len(filterSplit) < 2 {
				return errors.Errorf("filter input must be in the form of filter=value: %s is invalid", f)
			}
			generatedFunc, err := generateVolumeFilterFuncs(filterSplit[0], filterSplit[1])
			if err != nil {
				return errors.Wrapf(err, "invalid filter")
			}
			filterFuncs = append(filterFuncs, generatedFunc)
		}
	}

	volumes, err := runtime.Volumes(getContext())
	if err != nil {
		return err
	}
	// Get the volumes that match the filter
	volsFiltered := make([]*adapter.Volume, 0, len(volumes))
	for _, vol := range volumes {
		include := true
		for _, filter := range filterFuncs {
			include = include && filter(vol)
		}

		if include {
			volsFiltered = append(volsFiltered, vol)
		}
	}
	return generateVolLsOutput(volsFiltered, opts)
}

// generate the template based on conditions given
func genVolLsFormat(c *cliconfig.VolumeLsValues) string {
	var format string
	if c.Format != "" {
		// "\t" from the command line is not being recognized as a tab
		// replacing the string "\t" to a tab character if the user passes in "\t"
		format = strings.Replace(c.Format, `\t`, "\t", -1)
	}
	if c.Quiet {
		format = "{{.Name}}"
	}
	return format
}

// Convert output to genericParams for printing
func volLsToGeneric(templParams []volumeLsTemplateParams, JSONParams []volumeLsJSONParams) (genericParams []interface{}) {
	if len(templParams) > 0 {
		for _, v := range templParams {
			genericParams = append(genericParams, interface{}(v))
		}
		return
	}
	for _, v := range JSONParams {
		genericParams = append(genericParams, interface{}(v))
	}
	return
}

// generate the accurate header based on template given
func (vol *volumeLsTemplateParams) volHeaderMap() map[string]string {
	v := reflect.Indirect(reflect.ValueOf(vol))
	values := make(map[string]string)

	for i := 0; i < v.NumField(); i++ {
		key := v.Type().Field(i).Name
		value := key
		if value == "Name" {
			value = "Volume" + value
		}
		values[key] = strings.ToUpper(splitCamelCase(value))
	}
	return values
}

// getVolTemplateOutput returns all the volumes in the volumeLsTemplateParams format
func getVolTemplateOutput(lsParams []volumeLsJSONParams, opts volumeLsOptions) ([]volumeLsTemplateParams, error) {
	var lsOutput []volumeLsTemplateParams

	for _, lsParam := range lsParams {
		var (
			labels  string
			options string
		)

		for k, v := range lsParam.Labels {
			label := k
			if v != "" {
				label += "=" + v
			}
			labels += label
		}
		for k, v := range lsParam.Options {
			option := k
			if v != "" {
				option += "=" + v
			}
			options += option
		}
		params := volumeLsTemplateParams{
			Name:       lsParam.Name,
			Driver:     lsParam.Driver,
			MountPoint: lsParam.MountPoint,
			Scope:      lsParam.Scope,
			Labels:     labels,
			Options:    options,
		}

		lsOutput = append(lsOutput, params)
	}
	return lsOutput, nil
}

// getVolJSONParams returns the volumes in JSON format
func getVolJSONParams(volumes []*adapter.Volume) []volumeLsJSONParams {
	var lsOutput []volumeLsJSONParams

	for _, volume := range volumes {
		params := volumeLsJSONParams{
			Name:       volume.Name(),
			Labels:     volume.Labels(),
			MountPoint: volume.MountPoint(),
			Driver:     volume.Driver(),
			Options:    volume.Options(),
			Scope:      volume.Scope(),
		}

		lsOutput = append(lsOutput, params)
	}
	return lsOutput
}

// generateVolLsOutput generates the output based on the format, JSON or Go Template, and prints it out
func generateVolLsOutput(volumes []*adapter.Volume, opts volumeLsOptions) error {
	if len(volumes) == 0 && opts.Format != formats.JSONString {
		return nil
	}
	lsOutput := getVolJSONParams(volumes)
	var out formats.Writer

	switch opts.Format {
	case formats.JSONString:
		out = formats.JSONStructArray{Output: volLsToGeneric([]volumeLsTemplateParams{}, lsOutput)}
	default:
		lsOutput, err := getVolTemplateOutput(lsOutput, opts)
		if err != nil {
			return errors.Wrapf(err, "unable to create volume output")
		}
		out = formats.StdoutTemplateArray{Output: volLsToGeneric(lsOutput, []volumeLsJSONParams{}), Template: opts.Format, Fields: lsOutput[0].volHeaderMap()}
	}
	return formats.Writer(out).Out()
}

// generateVolumeFilterFuncs returns the true if the volume matches the filter set, otherwise it returns false.
func generateVolumeFilterFuncs(filter, filterValue string) (func(volume *adapter.Volume) bool, error) {
	switch filter {
	case "name":
		return func(v *adapter.Volume) bool {
			return strings.Contains(v.Name(), filterValue)
		}, nil
	case "driver":
		return func(v *adapter.Volume) bool {
			return v.Driver() == filterValue
		}, nil
	case "scope":
		return func(v *adapter.Volume) bool {
			return v.Scope() == filterValue
		}, nil
	case "label":
		filterArray := strings.SplitN(filterValue, "=", 2)
		filterKey := filterArray[0]
		if len(filterArray) > 1 {
			filterValue = filterArray[1]
		} else {
			filterValue = ""
		}
		return func(v *adapter.Volume) bool {
			for labelKey, labelValue := range v.Labels() {
				if labelKey == filterKey && ("" == filterValue || labelValue == filterValue) {
					return true
				}
			}
			return false
		}, nil
	case "opt":
		filterArray := strings.SplitN(filterValue, "=", 2)
		filterKey := filterArray[0]
		if len(filterArray) > 1 {
			filterValue = filterArray[1]
		} else {
			filterValue = ""
		}
		return func(v *adapter.Volume) bool {
			for labelKey, labelValue := range v.Options() {
				if labelKey == filterKey && ("" == filterValue || labelValue == filterValue) {
					return true
				}
			}
			return false
		}, nil
	}
	return nil, errors.Errorf("%s is an invalid filter", filter)
}
