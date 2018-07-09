package main

import (
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/cmd/podman/batchcontainer"
	"github.com/projectatomic/libpod/cmd/podman/formats"
	"github.com/projectatomic/libpod/cmd/podman/libpodruntime"
	"github.com/projectatomic/libpod/libpod"
	"github.com/urfave/cli"
)

var (
	opts batchcontainer.PsOptions
)

type podPsOptions struct {
	NoTrunc            bool
	Format             string
	Quiet              bool
	NumberOfContainers bool
}

type podPsTemplateParams struct {
	ID                 string
	Name               string
	NumberOfContainers int
}

// podPsJSONParams is used as a base structure for the psParams
// If template output is requested, podPsJSONParams will be converted to
// podPsTemplateParams.
// podPsJSONParams will be populated by data from libpod.Container,
// the members of the struct are the sama data types as their sources.
type podPsJSONParams struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	NumberOfContainers int    `json:"numberofcontainers"`
}

var (
	podPsFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "no-trunc",
			Usage: "Display the extended information",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Print the numeric IDs of the pods only",
		},
	}
	podPsDescription = "Prints out information about pods"
	podPsCommand     = cli.Command{
		Name:                   "ps",
		Aliases:                []string{"ls", "list"},
		Usage:                  "List pods",
		Description:            podPsDescription,
		Flags:                  podPsFlags,
		Action:                 podPsCmd,
		UseShortOptionHandling: true,
	}
)

func podPsCmd(c *cli.Context) error {
	if err := validateFlags(c, podPsFlags); err != nil {
		return err
	}

	if err := podCheckFlagsPassed(c); err != nil {
		return errors.Wrapf(err, "error with flags passed")
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}

	defer runtime.Shutdown(false)

	if len(c.Args()) > 0 {
		return errors.Errorf("too many arguments, ps takes no arguments")
	}

	format := genPodPsFormat(c.Bool("quiet"))

	opts := podPsOptions{
		Format:  format,
		NoTrunc: c.Bool("no-trunc"),
		Quiet:   c.Bool("quiet"),
	}

	var filterFuncs []libpod.PodFilter

	pods, err := runtime.Pods(filterFuncs...)
	if err != nil {
		return err
	}

	return generatePodPsOutput(pods, opts, runtime)
}

// podCheckFlagsPassed checks if mutually exclusive flags are passed together
func podCheckFlagsPassed(c *cli.Context) error {
	// quiet, and format with Go template are mutually exclusive
	flags := 0
	if c.Bool("quiet") {
		flags++
	}
	if flags > 1 {
		return errors.Errorf("quiet, and format with Go template are mutually exclusive")
	}
	return nil
}

// generate the template based on conditions given
func genPodPsFormat(quiet bool) string {
	if quiet {
		return formats.IDString
	}
	format := "table {{.ID}}\t{{.Name}}"
	return format
}

func podPsToGeneric(templParams []podPsTemplateParams, JSONParams []podPsJSONParams) (genericParams []interface{}) {
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
func (p *podPsTemplateParams) podHeaderMap() map[string]string {
	v := reflect.Indirect(reflect.ValueOf(p))
	values := make(map[string]string)

	for i := 0; i < v.NumField(); i++ {
		key := v.Type().Field(i).Name
		value := key
		if value == "ID" {
			value = "Pod" + value
		}
		values[key] = strings.ToUpper(splitCamelCase(value))
	}
	return values
}

// getPodTemplateOutput returns the modified container information
func getPodTemplateOutput(psParams []podPsJSONParams, opts podPsOptions) ([]podPsTemplateParams, error) {
	var (
		psOutput []podPsTemplateParams
	)

	for _, psParam := range psParams {
		podID := psParam.ID

		if !opts.NoTrunc {
			podID = shortID(psParam.ID)
		}
		params := podPsTemplateParams{
			ID:   podID,
			Name: psParam.Name,
		}

		psOutput = append(psOutput, params)
	}

	return psOutput, nil
}

// getAndSortPodJSONOutput returns the container info in its raw, sorted form
func getAndSortPodJSONParams(pods []*libpod.Pod, opts podPsOptions, runtime *libpod.Runtime) ([]podPsJSONParams, error) {
	var (
		psOutput []podPsJSONParams
	)

	for _, pod := range pods {
		ctrs, err := runtime.ContainersInPod(pod)
		if err != nil {
			return nil, err
		}
		ctrNum := len(ctrs)

		params := podPsJSONParams{
			ID:                 pod.ID(),
			Name:               pod.Name(),
			NumberOfContainers: ctrNum,
		}

		psOutput = append(psOutput, params)
	}
	return psOutput, nil
}

func generatePodPsOutput(pods []*libpod.Pod, opts podPsOptions, runtime *libpod.Runtime) error {
	if len(pods) == 0 && opts.Format != formats.JSONString {
		return nil
	}
	psOutput, err := getAndSortPodJSONParams(pods, opts, runtime)
	if err != nil {
		return err
	}
	var out formats.Writer

	switch opts.Format {
	case formats.JSONString:
		if err != nil {
			return errors.Wrapf(err, "unable to create JSON for output")
		}
		out = formats.JSONStructArray{Output: podPsToGeneric([]podPsTemplateParams{}, psOutput)}
	default:
		psOutput, err := getPodTemplateOutput(psOutput, opts)
		if err != nil {
			return errors.Wrapf(err, "unable to create output")
		}
		out = formats.StdoutTemplateArray{Output: podPsToGeneric(psOutput, []podPsJSONParams{}), Template: opts.Format, Fields: psOutput[0].podHeaderMap()}
	}

	return formats.Writer(out).Out()
}
