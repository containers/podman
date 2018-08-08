package main

import (
	"reflect"
	"strings"

	"github.com/containers/libpod/cmd/podman/formats"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
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

var volumeLsDescription = `
podman volume ls

List all available volumes. The output of the volumes can be filtered
and the output format can be changed to JSON or a user specified Go template.
`

var volumeLsFlags = []cli.Flag{
	cli.StringFlag{
		Name:  "filter, f",
		Usage: "Filter volume output",
	},
	cli.StringFlag{
		Name:  "format",
		Usage: "Format volume output using Go template",
		Value: "table {{.Driver}}\t{{.Name}}",
	},
	cli.BoolFlag{
		Name:  "quiet, q",
		Usage: "Print volume output in quiet mode",
	},
}

var volumeLsCommand = cli.Command{
	Name:                   "ls",
	Aliases:                []string{"list"},
	Usage:                  "List volumes",
	Description:            volumeLsDescription,
	Flags:                  volumeLsFlags,
	Action:                 volumeLsCmd,
	SkipArgReorder:         true,
	UseShortOptionHandling: true,
}

func volumeLsCmd(c *cli.Context) error {
	if err := validateFlags(c, volumeLsFlags); err != nil {
		return err
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	if len(c.Args()) > 0 {
		return errors.Errorf("too many arguments, ls takes no arguments")
	}

	opts := volumeLsOptions{
		Quiet: c.Bool("quiet"),
	}
	opts.Format = genVolLsFormat(c)

	// Get the filter functions based on any filters set
	var filterFuncs []libpod.VolumeFilter
	if c.String("filter") != "" {
		filters := strings.Split(c.String("filter"), ",")
		for _, f := range filters {
			filterSplit := strings.Split(f, "=")
			if len(filterSplit) < 2 {
				return errors.Errorf("filter input must be in the form of filter=value: %s is invalid", f)
			}
			generatedFunc, err := generateVolumeFilterFuncs(filterSplit[0], filterSplit[1], runtime)
			if err != nil {
				return errors.Wrapf(err, "invalid filter")
			}
			filterFuncs = append(filterFuncs, generatedFunc)
		}
	}

	volumes, err := runtime.GetAllVolumes()
	if err != nil {
		return err
	}

	// Get the volumes that match the filter
	volsFiltered := make([]*libpod.Volume, 0, len(volumes))
	for _, vol := range volumes {
		include := true
		for _, filter := range filterFuncs {
			include = include && filter(vol)
		}

		if include {
			volsFiltered = append(volsFiltered, vol)
		}
	}
	return generateVolLsOutput(volsFiltered, opts, runtime)
}

// generate the template based on conditions given
func genVolLsFormat(c *cli.Context) string {
	var format string
	if c.String("format") != "" {
		// "\t" from the command line is not being recognized as a tab
		// replacing the string "\t" to a tab character if the user passes in "\t"
		format = strings.Replace(c.String("format"), `\t`, "\t", -1)
	}
	if c.Bool("quiet") {
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
func getVolJSONParams(volumes []*libpod.Volume, opts volumeLsOptions, runtime *libpod.Runtime) ([]volumeLsJSONParams, error) {
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
	return lsOutput, nil
}

// generateVolLsOutput generates the output based on the format, JSON or Go Template, and prints it out
func generateVolLsOutput(volumes []*libpod.Volume, opts volumeLsOptions, runtime *libpod.Runtime) error {
	if len(volumes) == 0 && opts.Format != formats.JSONString {
		return nil
	}
	lsOutput, err := getVolJSONParams(volumes, opts, runtime)
	if err != nil {
		return err
	}
	var out formats.Writer

	switch opts.Format {
	case formats.JSONString:
		if err != nil {
			return errors.Wrapf(err, "unable to create JSON for volume output")
		}
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
func generateVolumeFilterFuncs(filter, filterValue string, runtime *libpod.Runtime) (func(volume *libpod.Volume) bool, error) {
	switch filter {
	case "name":
		return func(v *libpod.Volume) bool {
			return strings.Contains(v.Name(), filterValue)
		}, nil
	case "driver":
		return func(v *libpod.Volume) bool {
			return v.Driver() == filterValue
		}, nil
	case "scope":
		return func(v *libpod.Volume) bool {
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
		return func(v *libpod.Volume) bool {
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
		return func(v *libpod.Volume) bool {
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
