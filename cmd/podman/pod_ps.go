package main

import (
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/cmd/podman/batchcontainer"
	"github.com/projectatomic/libpod/cmd/podman/formats"
	"github.com/projectatomic/libpod/cmd/podman/libpodruntime"
	"github.com/projectatomic/libpod/libpod"
	"github.com/projectatomic/libpod/pkg/util"
	"github.com/urfave/cli"
	"k8s.io/apimachinery/pkg/fields"
)

var (
	opts batchcontainer.PsOptions
)

type podPsCtrInfo struct {
	Name   string `"json:name,omitempty"`
	Id     string `"json:id,omitempty"`
	Status string `"json:status,omitempty"`
}

type podPsOptions struct {
	Filter             string
	Format             string
	NoTrunc            bool
	Quiet              bool
	Sort               string
	Cgroup             bool
	Labels             bool
	NamesOfContainers  bool
	IdsOfContainers    bool
	NumberOfContainers bool
	StatusOfContainers bool
}

type podPsTemplateParams struct {
	ID                 string
	Name               string
	Labels             string
	Cgroup             string
	UsePodCgroup       bool
	ContainerInfo      string
	NumberOfContainers int
}

// podPsJSONParams is used as a base structure for the psParams
// If template output is requested, podPsJSONParams will be converted to
// podPsTemplateParams.
// podPsJSONParams will be populated by data from libpod.Container,
// the members of the struct are the sama data types as their sources.
type podPsJSONParams struct {
	ID                 string         `json:"id"`
	Name               string         `json:"name"`
	NumberOfContainers int            `json:"numberofcontainers"`
	CtrsInfo           []podPsCtrInfo `json:"containerinfo,omitempty"`
	Labels             fields.Set     `json:"labels,omitempty"`
	Cgroup             string         `json:"cgroup,omitempty"`
	UsePodCgroup       bool           `json:"podcgroup,omitempty"`
}

// Type declaration and functions for sorting the PS output
type podPsSorted []podPsJSONParams

func (a podPsSorted) Len() int      { return len(a) }
func (a podPsSorted) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

type podPsSortedId struct{ podPsSorted }

func (a podPsSortedId) Less(i, j int) bool { return a.podPsSorted[i].ID < a.podPsSorted[j].ID }

type podPsSortedNumber struct{ podPsSorted }

func (a podPsSortedNumber) Less(i, j int) bool {
	return len(a.podPsSorted[i].CtrsInfo) < len(a.podPsSorted[j].CtrsInfo)
}

type podPsSortedName struct{ podPsSorted }

func (a podPsSortedName) Less(i, j int) bool { return a.podPsSorted[i].Name < a.podPsSorted[j].Name }

var (
	podPsFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "ctr-names",
			Usage: "Display the container names",
		},
		cli.BoolFlag{
			Name:  "ctr-ids",
			Usage: "Display the container UUIDs. If no-trunc is not set they will be truncated",
		},
		cli.BoolFlag{
			Name:  "ctr-status",
			Usage: "Display the container status",
		},
		cli.StringFlag{
			Name:  "filter, f",
			Usage: "Filter output based on conditions given",
		},
		cli.StringFlag{
			Name:  "format",
			Usage: "Pretty-print pods to JSON or using a Go template",
		},
		cli.BoolFlag{
			Name:  "no-trunc",
			Usage: "Display the extended information",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Print the numeric IDs of the pods only",
		},
		cli.BoolFlag{
			Name:  "cgroup",
			Usage: "Print the Cgroup information of the pod",
		},
		cli.BoolFlag{
			Name:  "labels",
			Usage: "Print the labels of the pod",
		},
		cli.StringFlag{
			Name:  "sort",
			Usage: "Sort output by id, name, or number",
			Value: "name",
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

	format := genPodPsFormat(c.String("format"), c.Bool("quiet"), c.Bool("cgroup"), c.Bool("labels"), c.Bool("ctr-names"), c.Bool("ctr-ids"), c.Bool("ctr-status"))

	opts := podPsOptions{
		Filter:             c.String("filter"),
		Format:             format,
		NoTrunc:            c.Bool("no-trunc"),
		Quiet:              c.Bool("quiet"),
		Sort:               c.String("sort"),
		IdsOfContainers:    c.Bool("ctr-ids"),
		NamesOfContainers:  c.Bool("ctr-names"),
		StatusOfContainers: c.Bool("ctr-status"),
	}

	var filterFuncs []libpod.PodFilter
	if opts.Filter != "" {
		filters := strings.Split(opts.Filter, ",")
		for _, f := range filters {
			filterSplit := strings.Split(f, "=")
			if len(filterSplit) < 2 {
				return errors.Errorf("filter input must be in the form of filter=value: %s is invalid", f)
			}
			generatedFunc, err := generatePodFilterFuncs(filterSplit[0], filterSplit[1], runtime)
			if err != nil {
				return errors.Wrapf(err, "invalid filter")
			}
			filterFuncs = append(filterFuncs, generatedFunc)
		}
	}

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
	if c.IsSet("format") && c.String("format") != formats.JSONString {
		flags++
	}
	if flags > 1 {
		return errors.Errorf("quiet, and format with Go template are mutually exclusive")
	}
	return nil
}

func generatePodFilterFuncs(filter, filterValue string, runtime *libpod.Runtime) (func(pod *libpod.Pod) bool, error) {
	switch filter {
	case "id":
		return func(p *libpod.Pod) bool {
			return strings.Contains(p.ID(), filterValue)
		}, nil
	case "label":
		return func(p *libpod.Pod) bool {
			for _, label := range p.Labels() {
				if label == filterValue {
					return true
				}
			}
			return false
		}, nil
	case "name":
		return func(p *libpod.Pod) bool {
			return strings.Contains(p.Name(), filterValue)
		}, nil
	case "ctr-names":
		return func(p *libpod.Pod) bool {
			ctrNames, err := runtime.ContainerNamesInPod(p)
			if err != nil {
				return false
			}
			return util.StringInSlice(filterValue, ctrNames)
		}, nil
	case "ctr-ids":
		return func(p *libpod.Pod) bool {
			ctrIds, err := runtime.ContainerIDsInPod(p)
			if err != nil {
				return false
			}
			return util.StringInSlice(filterValue, ctrIds)
		}, nil
	case "ctr-number":
		return func(p *libpod.Pod) bool {
			ctrIds, err := runtime.ContainerIDsInPod(p)
			if err != nil {
				return false
			}

			fVint, err2 := strconv.Atoi(filterValue)
			if err2 != nil {
				return false
			}
			return len(ctrIds) == fVint
		}, nil
	case "ctr-status":
		if !util.StringInSlice(filterValue, []string{"created", "restarting", "running", "paused", "exited", "unknown"}) {
			return nil, errors.Errorf("%s is not a valid status", filterValue)
		}
		return func(p *libpod.Pod) bool {
			ctrs, err := runtime.ContainersInPod(p)
			if err != nil {
				return false
			}
			for _, ctr := range ctrs {
				status, err := ctr.State()
				if err != nil {
					return false
				}
				state := status.String()
				if status == libpod.ContainerStateConfigured {
					state = "created"
				}
				if state == filterValue {
					return true
				}
			}
			return false
		}, nil
	}
	return nil, errors.Errorf("%s is an invalid filter", filter)
}

// generate the template based on conditions given
func genPodPsFormat(format string, quiet, cgroup, labels, names, ids, status bool) string {
	if format != "" {
		// "\t" from the command line is not being recognized as a tab
		// replacing the string "\t" to a tab character if the user passes in "\t"
		return strings.Replace(format, `\t`, "\t", -1)
	}
	if quiet {
		return formats.IDString
	}
	format = "table {{.ID}}\t{{.Name}}"
	if names || ids || status {
		format += "\t{{.ContainerInfo}}"
	} else {
		format += "\t{{.NumberOfContainers}}"
	}
	if labels {
		format += "\t{{.Labels}}"
	}
	if cgroup {
		format += "\t{{.Cgroup}}\t{{.UsePodCgroup}}"
	}
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

func sortPodPsOutput(sortBy string, psOutput podPsSorted) (podPsSorted, error) {
	switch sortBy {
	case "id":
		sort.Sort(podPsSortedId{psOutput})
	case "name":
		sort.Sort(podPsSortedName{psOutput})
	case "number":
		sort.Sort(podPsSortedNumber{psOutput})
	default:
		return nil, errors.Errorf("invalid option for --sort, options are: id, names, or number")
	}
	return psOutput, nil
}

// getPodTemplateOutput returns the modified container information
func getPodTemplateOutput(psParams []podPsJSONParams, opts podPsOptions) ([]podPsTemplateParams, error) {
	var (
		psOutput []podPsTemplateParams
	)

	for _, psParam := range psParams {
		labels := formatLabels(psParam.Labels)

		podID := psParam.ID

		var ctrStr string
		if !opts.NoTrunc {
			podID = shortID(psParam.ID)
			if opts.IdsOfContainers {
				for i := 0; i < len(psParam.CtrsInfo); i++ {
					psParam.CtrsInfo[i].Id = shortID(psParam.CtrsInfo[i].Id)
				}
			}
		}
		for _, ctrInfo := range psParam.CtrsInfo {
			ctrStr += "[ "
			if opts.IdsOfContainers {
				ctrStr += ctrInfo.Id + " "
			}
			if opts.NamesOfContainers {
				ctrStr += ctrInfo.Name + " "
			}
			if opts.StatusOfContainers {
				ctrStr += ctrInfo.Status + " "
			}
			ctrStr += "] "
		}
		params := podPsTemplateParams{
			ID:                 podID,
			Name:               psParam.Name,
			Labels:             labels,
			UsePodCgroup:       psParam.UsePodCgroup,
			Cgroup:             psParam.Cgroup,
			ContainerInfo:      ctrStr,
			NumberOfContainers: psParam.NumberOfContainers,
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
	// bc_opts will be used to run a batch container,
	// the function only looks for size and namespace bools, neither of which
	// are needed for pod ps
	bc_opts := batchcontainer.PsOptions{}
	for _, pod := range pods {
		ctrs, err := runtime.ContainersInPod(pod)
		ctrsInfo := make([]podPsCtrInfo, 0)
		if err != nil {
			return nil, err
		}
		for _, ctr := range ctrs {
			batchInfo, err := batchcontainer.BatchContainerOp(ctr, bc_opts)
			if err != nil {
				return nil, err
			}
			var status string
			switch batchInfo.ConState {
			case libpod.ContainerStateStopped:
				status = "Exited"
			case libpod.ContainerStateRunning:
				status = "Running"
			case libpod.ContainerStatePaused:
				status = "Paused"
			case libpod.ContainerStateCreated, libpod.ContainerStateConfigured:
				status = "Created"
			default:
				status = "Dead"
			}
			ctrsInfo = append(ctrsInfo, podPsCtrInfo{
				Name:   batchInfo.ConConfig.Name,
				Id:     ctr.ID(),
				Status: status,
			})
		}

		ctrNum := len(ctrs)

		params := podPsJSONParams{
			ID:                 pod.ID(),
			Name:               pod.Name(),
			Labels:             pod.Labels(),
			Cgroup:             pod.CgroupParent(),
			UsePodCgroup:       pod.UsePodCgroup(),
			NumberOfContainers: ctrNum,
			CtrsInfo:           ctrsInfo,
		}

		psOutput = append(psOutput, params)
	}
	return sortPodPsOutput(opts.Sort, psOutput)
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
