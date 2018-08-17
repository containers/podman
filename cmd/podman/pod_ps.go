package main

import (
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/containers/libpod/cmd/podman/formats"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/util"
	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

const (
	STOPPED      = "Stopped"
	RUNNING      = "Running"
	PAUSED       = "Paused"
	EXITED       = "Exited"
	ERROR        = "Error"
	CREATED      = "Created"
	NUM_CTR_INFO = 10
)

var (
	bc_opts shared.PsOptions
)

type podPsCtrInfo struct {
	Name   string `"json:name,omitempty"`
	Id     string `"json:id,omitempty"`
	Status string `"json:status,omitempty"`
}

type podPsOptions struct {
	NoTrunc            bool
	Format             string
	Sort               string
	Quiet              bool
	NumberOfContainers bool
	Cgroup             bool
	NamesOfContainers  bool
	IdsOfContainers    bool
	StatusOfContainers bool
}

type podPsTemplateParams struct {
	Created            string
	ID                 string
	Name               string
	NumberOfContainers int
	Status             string
	Cgroup             string
	ContainerInfo      string
	InfraContainerID   string
	SharedNamespaces   string
}

// podPsJSONParams is used as a base structure for the psParams
// If template output is requested, podPsJSONParams will be converted to
// podPsTemplateParams.
// podPsJSONParams will be populated by data from libpod.Container,
// the members of the struct are the sama data types as their sources.
type podPsJSONParams struct {
	CreatedAt          time.Time      `json:"createdAt"`
	ID                 string         `json:"id"`
	Name               string         `json:"name"`
	NumberOfContainers int            `json:"numberofcontainers"`
	Status             string         `json:"status"`
	CtrsInfo           []podPsCtrInfo `json:"containerinfo,omitempty"`
	Cgroup             string         `json:"cgroup,omitempty"`
	InfraContainerID   string         `json:"infracontainerid,omitempty"`
	SharedNamespaces   []string       `json:"sharednamespaces,omitempty"`
}

// Type declaration and functions for sorting the pod PS output
type podPsSorted []podPsJSONParams

func (a podPsSorted) Len() int      { return len(a) }
func (a podPsSorted) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

type podPsSortedCreated struct{ podPsSorted }

func (a podPsSortedCreated) Less(i, j int) bool {
	return a.podPsSorted[i].CreatedAt.After(a.podPsSorted[j].CreatedAt)
}

type podPsSortedId struct{ podPsSorted }

func (a podPsSortedId) Less(i, j int) bool { return a.podPsSorted[i].ID < a.podPsSorted[j].ID }

type podPsSortedNumber struct{ podPsSorted }

func (a podPsSortedNumber) Less(i, j int) bool {
	return len(a.podPsSorted[i].CtrsInfo) < len(a.podPsSorted[j].CtrsInfo)
}

type podPsSortedName struct{ podPsSorted }

func (a podPsSortedName) Less(i, j int) bool { return a.podPsSorted[i].Name < a.podPsSorted[j].Name }

type podPsSortedStatus struct{ podPsSorted }

func (a podPsSortedStatus) Less(i, j int) bool {
	return a.podPsSorted[i].Status < a.podPsSorted[j].Status
}

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
			Name:  "latest, l",
			Usage: "Show the latest pod created",
		},
		cli.BoolFlag{
			Name:  "namespace, ns",
			Usage: "Display namespace information of the pod",
		},
		cli.BoolFlag{
			Name:  "no-trunc",
			Usage: "Do not truncate pod and container IDs",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Print the numeric IDs of the pods only",
		},
		cli.StringFlag{
			Name:  "sort",
			Usage: "Sort output by created, id, name, or number",
			Value: "created",
		},
	}
	podPsDescription = "List all pods on system including their names, ids and current state."
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

	if err := podPsCheckFlagsPassed(c); err != nil {
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

	opts := podPsOptions{
		NoTrunc:            c.Bool("no-trunc"),
		Quiet:              c.Bool("quiet"),
		Sort:               c.String("sort"),
		IdsOfContainers:    c.Bool("ctr-ids"),
		NamesOfContainers:  c.Bool("ctr-names"),
		StatusOfContainers: c.Bool("ctr-status"),
	}

	opts.Format = genPodPsFormat(c)

	var filterFuncs []libpod.PodFilter
	if c.String("filter") != "" {
		filters := strings.Split(c.String("filter"), ",")
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

	var pods []*libpod.Pod
	if c.IsSet("latest") {
		pod, err := runtime.GetLatestPod()
		if err != nil {
			return err
		}
		pods = append(pods, pod)
	} else {
		pods, err = runtime.GetAllPods()
		if err != nil {
			return err
		}
	}

	podsFiltered := make([]*libpod.Pod, 0, len(pods))
	for _, pod := range pods {
		include := true
		for _, filter := range filterFuncs {
			include = include && filter(pod)
		}

		if include {
			podsFiltered = append(podsFiltered, pod)
		}
	}

	return generatePodPsOutput(podsFiltered, opts, runtime)
}

// podPsCheckFlagsPassed checks if mutually exclusive flags are passed together
func podPsCheckFlagsPassed(c *cli.Context) error {
	// quiet, and format with Go template are mutually exclusive
	flags := 0
	if c.Bool("quiet") {
		flags++
	}
	if c.IsSet("format") && c.String("format") != formats.JSONString {
		flags++
	}
	if flags > 1 {
		return errors.Errorf("quiet and format with Go template are mutually exclusive")
	}
	return nil
}

func generatePodFilterFuncs(filter, filterValue string, runtime *libpod.Runtime) (func(pod *libpod.Pod) bool, error) {
	switch filter {
	case "ctr-ids":
		return func(p *libpod.Pod) bool {
			ctrIds, err := p.AllContainersByID()
			if err != nil {
				return false
			}
			return util.StringInSlice(filterValue, ctrIds)
		}, nil
	case "ctr-names":
		return func(p *libpod.Pod) bool {
			ctrs, err := p.AllContainers()
			if err != nil {
				return false
			}
			for _, ctr := range ctrs {
				if filterValue == ctr.Name() {
					return true
				}
			}
			return false
		}, nil
	case "ctr-number":
		return func(p *libpod.Pod) bool {
			ctrIds, err := p.AllContainersByID()
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
			ctr_statuses, err := p.Status()
			if err != nil {
				return false
			}
			for _, ctr_status := range ctr_statuses {
				state := ctr_status.String()
				if ctr_status == libpod.ContainerStateConfigured {
					state = "created"
				}
				if state == filterValue {
					return true
				}
			}
			return false
		}, nil
	case "id":
		return func(p *libpod.Pod) bool {
			return strings.Contains(p.ID(), filterValue)
		}, nil
	case "name":
		return func(p *libpod.Pod) bool {
			return strings.Contains(p.Name(), filterValue)
		}, nil
	case "status":
		if !util.StringInSlice(filterValue, []string{"stopped", "running", "paused", "exited", "dead", "created"}) {
			return nil, errors.Errorf("%s is not a valid pod status", filterValue)
		}
		return func(p *libpod.Pod) bool {
			status, err := shared.GetPodStatus(p)
			if err != nil {
				return false
			}
			if strings.ToLower(status) == filterValue {
				return true
			}
			return false
		}, nil
	}
	return nil, errors.Errorf("%s is an invalid filter", filter)
}

// generate the template based on conditions given
func genPodPsFormat(c *cli.Context) string {
	format := ""
	if c.String("format") != "" {
		// "\t" from the command line is not being recognized as a tab
		// replacing the string "\t" to a tab character if the user passes in "\t"
		format = strings.Replace(c.String("format"), `\t`, "\t", -1)
	} else if c.Bool("quiet") {
		format = formats.IDString
	} else {
		format = "table {{.ID}}\t{{.Name}}\t{{.Status}}\t{{.Created}}"
		if c.Bool("namespace") {
			format += "\t{{.Cgroup}}\t{{.SharedNamespaces}}"
		}
		if c.Bool("ctr-names") || c.Bool("ctr-ids") || c.Bool("ctr-status") {
			format += "\t{{.ContainerInfo}}"
		} else {
			format += "\t{{.NumberOfContainers}}"
		}
		format += "\t{{.InfraContainerID}}"
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
	case "created":
		sort.Sort(podPsSortedCreated{psOutput})
	case "id":
		sort.Sort(podPsSortedId{psOutput})
	case "name":
		sort.Sort(podPsSortedName{psOutput})
	case "number":
		sort.Sort(podPsSortedNumber{psOutput})
	case "status":
		sort.Sort(podPsSortedStatus{psOutput})
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
		podID := psParam.ID
		infraID := psParam.InfraContainerID
		var ctrStr string

		truncated := ""
		if !opts.NoTrunc {
			podID = shortID(podID)
			if len(psParam.CtrsInfo) > NUM_CTR_INFO {
				psParam.CtrsInfo = psParam.CtrsInfo[:NUM_CTR_INFO]
				truncated = "..."
			}
			infraID = shortID(infraID)
		}
		for _, ctrInfo := range psParam.CtrsInfo {
			ctrStr += "[ "
			if opts.IdsOfContainers {
				if opts.NoTrunc {
					ctrStr += ctrInfo.Id
				} else {
					ctrStr += shortID(ctrInfo.Id)
				}
			}
			if opts.NamesOfContainers {
				ctrStr += ctrInfo.Name + " "
			}
			if opts.StatusOfContainers {
				ctrStr += ctrInfo.Status + " "
			}
			ctrStr += "] "
		}
		ctrStr += truncated
		params := podPsTemplateParams{
			Created:            units.HumanDuration(time.Since(psParam.CreatedAt)) + " ago",
			ID:                 podID,
			Name:               psParam.Name,
			Status:             psParam.Status,
			NumberOfContainers: psParam.NumberOfContainers,
			Cgroup:             psParam.Cgroup,
			ContainerInfo:      ctrStr,
			InfraContainerID:   infraID,
			SharedNamespaces:   strings.Join(psParam.SharedNamespaces, ","),
		}

		psOutput = append(psOutput, params)
	}

	return psOutput, nil
}

func getSharedNamespaces(pod *libpod.Pod) []string {
	var shared []string
	if pod.SharesPID() {
		shared = append(shared, "pid")
	}
	if pod.SharesNet() {
		shared = append(shared, "net")
	}
	if pod.SharesMNT() {
		shared = append(shared, "mnt")
	}
	if pod.SharesIPC() {
		shared = append(shared, "ipc")
	}
	if pod.SharesUser() {
		shared = append(shared, "user")
	}
	if pod.SharesCgroup() {
		shared = append(shared, "cgroup")
	}
	if pod.SharesUTS() {
		shared = append(shared, "uts")
	}
	return shared
}

// getAndSortPodJSONOutput returns the container info in its raw, sorted form
func getAndSortPodJSONParams(pods []*libpod.Pod, opts podPsOptions, runtime *libpod.Runtime) ([]podPsJSONParams, error) {
	var (
		psOutput []podPsJSONParams
	)

	for _, pod := range pods {
		ctrs, err := pod.AllContainers()
		ctrsInfo := make([]podPsCtrInfo, 0)
		if err != nil {
			return nil, err
		}
		ctrNum := len(ctrs)
		status, err := shared.GetPodStatus(pod)
		if err != nil {
			return nil, err
		}

		infraContainerID, err := pod.InfraContainerID()
		if err != nil {
			return nil, err
		}
		for _, ctr := range ctrs {
			batchInfo, err := shared.BatchContainerOp(ctr, bc_opts)
			if err != nil {
				return nil, err
			}
			var status string
			switch batchInfo.ConState {
			case libpod.ContainerStateStopped:
				status = EXITED
			case libpod.ContainerStateRunning:
				status = RUNNING
			case libpod.ContainerStatePaused:
				status = PAUSED
			case libpod.ContainerStateCreated, libpod.ContainerStateConfigured:
				status = CREATED
			default:
				status = ERROR
			}
			ctrsInfo = append(ctrsInfo, podPsCtrInfo{
				Name:   batchInfo.ConConfig.Name,
				Id:     ctr.ID(),
				Status: status,
			})
		}
		params := podPsJSONParams{
			CreatedAt:          pod.CreatedTime(),
			ID:                 pod.ID(),
			Name:               pod.Name(),
			Status:             status,
			Cgroup:             pod.CgroupParent(),
			NumberOfContainers: ctrNum,
			CtrsInfo:           ctrsInfo,
			SharedNamespaces:   getSharedNamespaces(pod),
			InfraContainerID:   infraContainerID,
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
