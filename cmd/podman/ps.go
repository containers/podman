package main

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/containers/libpod/cmd/podman/batchcontainer"
	"github.com/containers/libpod/cmd/podman/formats"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/util"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"k8s.io/apimachinery/pkg/fields"
)

const mountTruncLength = 12

type psTemplateParams struct {
	ID            string
	Image         string
	Command       string
	CreatedAtTime time.Time
	Created       string
	Status        string
	Ports         string
	Size          string
	Names         string
	Labels        string
	Mounts        string
	PID           int
	Cgroup        string
	IPC           string
	MNT           string
	NET           string
	PIDNS         string
	User          string
	UTS           string
	Pod           string
}

// psJSONParams is used as a base structure for the psParams
// If template output is requested, psJSONParams will be converted to
// psTemplateParams.
// psJSONParams will be populated by data from libpod.Container,
// the members of the struct are the sama data types as their sources.
type psJSONParams struct {
	ID               string                        `json:"id"`
	Image            string                        `json:"image"`
	ImageID          string                        `json:"image_id"`
	Command          []string                      `json:"command"`
	ExitCode         int32                         `json:"exitCode"`
	Exited           bool                          `json:"exited"`
	CreatedAt        time.Time                     `json:"createdAt"`
	StartedAt        time.Time                     `json:"startedAt"`
	ExitedAt         time.Time                     `json:"exitedAt"`
	Status           string                        `json:"status"`
	PID              int                           `json:"PID"`
	Ports            []ocicni.PortMapping          `json:"ports"`
	Size             *batchcontainer.ContainerSize `json:"size,omitempty"`
	Names            string                        `json:"names"`
	Labels           fields.Set                    `json:"labels"`
	Mounts           []string                      `json:"mounts"`
	ContainerRunning bool                          `json:"ctrRunning"`
	Namespaces       *batchcontainer.Namespace     `json:"namespace,omitempty"`
	Pod              string                        `json:"pod,omitempty"`
}

// Type declaration and functions for sorting the PS output
type psSorted []psJSONParams

func (a psSorted) Len() int      { return len(a) }
func (a psSorted) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

type psSortedCommand struct{ psSorted }

func (a psSortedCommand) Less(i, j int) bool {
	return strings.Join(a.psSorted[i].Command, " ") < strings.Join(a.psSorted[j].Command, " ")
}

type psSortedCreated struct{ psSorted }

func (a psSortedCreated) Less(i, j int) bool {
	return a.psSorted[i].CreatedAt.After(a.psSorted[j].CreatedAt)
}

type psSortedId struct{ psSorted }

func (a psSortedId) Less(i, j int) bool { return a.psSorted[i].ID < a.psSorted[j].ID }

type psSortedImage struct{ psSorted }

func (a psSortedImage) Less(i, j int) bool { return a.psSorted[i].Image < a.psSorted[j].Image }

type psSortedNames struct{ psSorted }

func (a psSortedNames) Less(i, j int) bool { return a.psSorted[i].Names < a.psSorted[j].Names }

type psSortedPod struct{ psSorted }

func (a psSortedPod) Less(i, j int) bool { return a.psSorted[i].Pod < a.psSorted[j].Pod }

type psSortedRunningFor struct{ psSorted }

func (a psSortedRunningFor) Less(i, j int) bool {
	return a.psSorted[j].StartedAt.After(a.psSorted[i].StartedAt)
}

type psSortedStatus struct{ psSorted }

func (a psSortedStatus) Less(i, j int) bool { return a.psSorted[i].Status < a.psSorted[j].Status }

type psSortedSize struct{ psSorted }

func (a psSortedSize) Less(i, j int) bool {
	if a.psSorted[i].Size == nil || a.psSorted[j].Size == nil {
		return false
	}
	return a.psSorted[i].Size.RootFsSize < a.psSorted[j].Size.RootFsSize
}

var (
	psFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "Show all the containers, default is only running containers",
		},
		cli.StringFlag{
			Name:  "filter, f",
			Usage: "Filter output based on conditions given",
		},
		cli.StringFlag{
			Name:  "format",
			Usage: "Pretty-print containers to JSON or using a Go template",
		},
		cli.IntFlag{
			Name:  "last, n",
			Usage: "Print the n last created containers (all states)",
			Value: -1,
		},
		cli.BoolFlag{
			Name:  "latest, l",
			Usage: "Show the latest container created (all states)",
		},
		cli.BoolFlag{
			Name:  "namespace, ns",
			Usage: "Display namespace information",
		},
		cli.BoolFlag{
			Name:  "no-trunc",
			Usage: "Display the extended information",
		},
		cli.BoolFlag{
			Name:  "pod",
			Usage: "Print the ID and name of the pod the containers are associated with",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Print the numeric IDs of the containers only",
		},
		cli.BoolFlag{
			Name:  "size, s",
			Usage: "Display the total file sizes",
		},
		cli.StringFlag{
			Name:  "sort",
			Usage: "Sort output by command, created, id, image, names, runningfor, size, or status",
			Value: "created",
		},
	}
	psDescription = "Prints out information about the containers"
	psCommand     = cli.Command{
		Name:                   "ps",
		Usage:                  "List containers",
		Description:            psDescription,
		Flags:                  psFlags,
		Action:                 psCmd,
		ArgsUsage:              "",
		UseShortOptionHandling: true,
	}
	lsCommand = cli.Command{
		Name:                   "ls",
		Usage:                  "List containers",
		Description:            psDescription,
		Flags:                  psFlags,
		Action:                 psCmd,
		ArgsUsage:              "",
		UseShortOptionHandling: true,
	}
)

func psCmd(c *cli.Context) error {
	if err := validateFlags(c, psFlags); err != nil {
		return err
	}

	if err := checkFlagsPassed(c); err != nil {
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

	format := genPsFormat(c.String("format"), c.Bool("quiet"), c.Bool("size"), c.Bool("namespace"), c.Bool("pod"))

	opts := batchcontainer.PsOptions{
		All:       c.Bool("all"),
		Filter:    c.String("filter"),
		Format:    format,
		Last:      c.Int("last"),
		Latest:    c.Bool("latest"),
		NoTrunc:   c.Bool("no-trunc"),
		Pod:       c.Bool("pod"),
		Quiet:     c.Bool("quiet"),
		Size:      c.Bool("size"),
		Namespace: c.Bool("namespace"),
		Sort:      c.String("sort"),
	}

	var filterFuncs []libpod.ContainerFilter
	// When we are dealing with latest or last=n, we need to
	// get all containers.
	if !opts.All && !opts.Latest && opts.Last < 1 {
		// only get running containers
		filterFuncs = append(filterFuncs, func(c *libpod.Container) bool {
			state, _ := c.State()
			return state == libpod.ContainerStateRunning
		})
	}

	if opts.Filter != "" {
		filters := strings.Split(opts.Filter, ",")
		for _, f := range filters {
			filterSplit := strings.SplitN(f, "=", 2)
			if len(filterSplit) < 2 {
				return errors.Errorf("filter input must be in the form of filter=value: %s is invalid", f)
			}
			generatedFunc, err := generateContainerFilterFuncs(filterSplit[0], filterSplit[1], runtime)
			if err != nil {
				return errors.Wrapf(err, "invalid filter")
			}
			filterFuncs = append(filterFuncs, generatedFunc)
		}
	}

	var outputContainers []*libpod.Container

	if !opts.Latest {
		// Get all containers
		containers, err := runtime.GetContainers(filterFuncs...)
		if err != nil {
			return err
		}

		// We only want the last few containers
		if opts.Last > 0 && opts.Last <= len(containers) {
			return errors.Errorf("--last not yet supported")
		} else {
			outputContainers = containers
		}
	} else {
		// Get just the latest container
		// Ignore filters
		latestCtr, err := runtime.GetLatestContainer()
		if err != nil {
			return err
		}

		outputContainers = []*libpod.Container{latestCtr}
	}

	return generatePsOutput(outputContainers, opts)
}

// checkFlagsPassed checks if mutually exclusive flags are passed together
func checkFlagsPassed(c *cli.Context) error {
	// latest, and last are mutually exclusive.
	if c.Int("last") >= 0 && c.Bool("latest") {
		return errors.Errorf("last and latest are mutually exclusive")
	}
	// Quiet conflicts with size, namespace, and format with a Go template
	if c.Bool("quiet") {
		if c.Bool("size") || c.Bool("namespace") || (c.IsSet("format") &&
			c.String("format") != formats.JSONString) {
			return errors.Errorf("quiet conflicts with size, namespace, and format with go template")
		}
	}
	// Size and namespace conflict with each other
	if c.Bool("size") && c.Bool("namespace") {
		return errors.Errorf("size and namespace options conflict")
	}
	return nil
}

func generateContainerFilterFuncs(filter, filterValue string, runtime *libpod.Runtime) (func(container *libpod.Container) bool, error) {
	switch filter {
	case "id":
		return func(c *libpod.Container) bool {
			return strings.Contains(c.ID(), filterValue)
		}, nil
	case "label":
		var filterArray []string = strings.Split(filterValue, "=")
		var filterKey string = filterArray[0]
		if len(filterArray) > 1 {
			filterValue = filterArray[1]
		} else {
			filterValue = ""
		}
		return func(c *libpod.Container) bool {
			for labelKey, labelValue := range c.Labels() {
				if labelKey == filterKey && ("" == filterValue || labelValue == filterValue) {
					return true
				}
			}
			return false
		}, nil
	case "name":
		return func(c *libpod.Container) bool {
			return strings.Contains(c.Name(), filterValue)
		}, nil
	case "exited":
		exitCode, err := strconv.ParseInt(filterValue, 10, 32)
		if err != nil {
			return nil, errors.Wrapf(err, "exited code out of range %q", filterValue)
		}
		return func(c *libpod.Container) bool {
			ec, exited, err := c.ExitCode()
			if ec == int32(exitCode) && err == nil && exited == true {
				return true
			}
			return false
		}, nil
	case "status":
		if !util.StringInSlice(filterValue, []string{"created", "restarting", "running", "paused", "exited", "unknown"}) {
			return nil, errors.Errorf("%s is not a valid status", filterValue)
		}
		return func(c *libpod.Container) bool {
			status, err := c.State()
			if err != nil {
				return false
			}
			state := status.String()
			if status == libpod.ContainerStateConfigured {
				state = "created"
			}
			return state == filterValue
		}, nil
	case "ancestor":
		// This needs to refine to match docker
		// - ancestor=(<image-name>[:tag]|<image-id>| ⟨image@digest⟩) - containers created from an image or a descendant.
		return func(c *libpod.Container) bool {
			containerConfig := c.Config()
			if strings.Contains(containerConfig.RootfsImageID, filterValue) || strings.Contains(containerConfig.RootfsImageName, filterValue) {
				return true
			}
			return false
		}, nil
	case "before":
		ctr, err := runtime.LookupContainer(filterValue)
		if err != nil {
			return nil, errors.Errorf("unable to find container by name or id of %s", filterValue)
		}
		containerConfig := ctr.Config()
		createTime := containerConfig.CreatedTime
		return func(c *libpod.Container) bool {
			cc := c.Config()
			return createTime.After(cc.CreatedTime)
		}, nil
	case "since":
		ctr, err := runtime.LookupContainer(filterValue)
		if err != nil {
			return nil, errors.Errorf("unable to find container by name or id of %s", filterValue)
		}
		containerConfig := ctr.Config()
		createTime := containerConfig.CreatedTime
		return func(c *libpod.Container) bool {
			cc := c.Config()
			return createTime.Before(cc.CreatedTime)
		}, nil
	case "volume":
		//- volume=(<volume-name>|<mount-point-destination>)
		return func(c *libpod.Container) bool {
			containerConfig := c.Config()
			var dest string
			arr := strings.Split(filterValue, ":")
			source := arr[0]
			if len(arr) == 2 {
				dest = arr[1]
			}
			for _, mount := range containerConfig.Spec.Mounts {
				if dest != "" && (mount.Source == source && mount.Destination == dest) {
					return true
				}
				if dest == "" && mount.Source == source {
					return true
				}
			}
			return false
		}, nil
	}
	return nil, errors.Errorf("%s is an invalid filter", filter)
}

// generate the template based on conditions given
func genPsFormat(format string, quiet, size, namespace, pod bool) string {
	if format != "" {
		// "\t" from the command line is not being recognized as a tab
		// replacing the string "\t" to a tab character if the user passes in "\t"
		return strings.Replace(format, `\t`, "\t", -1)
	}
	if quiet {
		return formats.IDString
	}
	podappend := ""
	if pod {
		podappend = "{{.Pod}}\t"
	}
	if namespace {
		return fmt.Sprintf("table {{.ID}}\t{{.Names}}\t%s{{.PID}}\t{{.Cgroup}}\t{{.IPC}}\t{{.MNT}}\t{{.NET}}\t{{.PIDNS}}\t{{.User}}\t{{.UTS}}\t", podappend)
	}
	format = "table {{.ID}}\t{{.Image}}\t{{.Command}}\t{{.Created}}\t{{.Status}}\t{{.Ports}}\t{{.Names}}\t"
	format += podappend
	if size {
		format += "{{.Size}}\t"
	}
	return format
}

func psToGeneric(templParams []psTemplateParams, JSONParams []psJSONParams) (genericParams []interface{}) {
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
func (p *psTemplateParams) headerMap() map[string]string {
	v := reflect.Indirect(reflect.ValueOf(p))
	values := make(map[string]string)

	for i := 0; i < v.NumField(); i++ {
		key := v.Type().Field(i).Name
		value := key
		if value == "ID" {
			value = "Container" + value
		}
		values[key] = strings.ToUpper(splitCamelCase(value))
	}
	return values
}

func sortPsOutput(sortBy string, psOutput psSorted) (psSorted, error) {
	switch sortBy {
	case "id":
		sort.Sort(psSortedId{psOutput})
	case "image":
		sort.Sort(psSortedImage{psOutput})
	case "command":
		sort.Sort(psSortedCommand{psOutput})
	case "runningfor":
		sort.Sort(psSortedRunningFor{psOutput})
	case "status":
		sort.Sort(psSortedStatus{psOutput})
	case "size":
		sort.Sort(psSortedSize{psOutput})
	case "names":
		sort.Sort(psSortedNames{psOutput})
	case "created":
		sort.Sort(psSortedCreated{psOutput})
	case "pod":
		sort.Sort(psSortedPod{psOutput})
	default:
		return nil, errors.Errorf("invalid option for --sort, options are: command, created, id, image, names, runningfor, size, or status")
	}
	return psOutput, nil
}

// getTemplateOutput returns the modified container information
func getTemplateOutput(psParams []psJSONParams, opts batchcontainer.PsOptions) ([]psTemplateParams, error) {
	var (
		psOutput          []psTemplateParams
		pod, status, size string
		ns                *batchcontainer.Namespace
	)
	// If the user is trying to filter based on size, or opted to sort on size
	// the size bool must be set.
	if strings.Contains(opts.Format, ".Size") || opts.Sort == "size" {
		opts.Size = true
	}
	if strings.Contains(opts.Format, ".Pod") || opts.Sort == "pod" {
		opts.Pod = true
	}

	for _, psParam := range psParams {
		// do we need this?
		imageName := psParam.Image
		ctrID := psParam.ID

		if opts.Namespace {
			ns = psParam.Namespaces
		}
		if opts.Size {
			if psParam.Size == nil {
				return nil, errors.Errorf("Container %s does not have a size struct", psParam.ID)
			}
			size = units.HumanSizeWithPrecision(float64(psParam.Size.RwSize), 3) + " (virtual " + units.HumanSizeWithPrecision(float64(psParam.Size.RootFsSize), 3) + ")"
		}
		if opts.Pod {
			pod = psParam.Pod
		}

		command := strings.Join(psParam.Command, " ")
		if !opts.NoTrunc {
			if len(command) > 20 {
				command = command[:19] + "..."
			}
		}
		ports := portsToString(psParam.Ports)
		labels := formatLabels(psParam.Labels)

		switch psParam.Status {
		case libpod.ContainerStateStopped.String():
			exitedSince := units.HumanDuration(time.Since(psParam.ExitedAt))
			status = fmt.Sprintf("Exited (%d) %s ago", psParam.ExitCode, exitedSince)
		case libpod.ContainerStateRunning.String():
			status = "Up " + units.HumanDuration(time.Since(psParam.StartedAt)) + " ago"
		case libpod.ContainerStatePaused.String():
			status = "Paused"
		case libpod.ContainerStateCreated.String(), libpod.ContainerStateConfigured.String():
			status = "Created"
		default:
			status = "Error"
		}

		if !opts.NoTrunc {
			ctrID = shortID(psParam.ID)
			pod = shortID(psParam.Pod)
		}
		params := psTemplateParams{
			ID:            ctrID,
			Image:         imageName,
			Command:       command,
			CreatedAtTime: psParam.CreatedAt,
			Created:       units.HumanDuration(time.Since(psParam.CreatedAt)) + " ago",
			Status:        status,
			Ports:         ports,
			Size:          size,
			Names:         psParam.Names,
			Labels:        labels,
			Mounts:        getMounts(psParam.Mounts, opts.NoTrunc),
			PID:           psParam.PID,
			Pod:           pod,
		}

		if opts.Namespace {
			params.Cgroup = ns.Cgroup
			params.IPC = ns.IPC
			params.MNT = ns.MNT
			params.NET = ns.NET
			params.PIDNS = ns.PIDNS
			params.User = ns.User
			params.UTS = ns.UTS
		}
		psOutput = append(psOutput, params)
	}

	return psOutput, nil
}

// getAndSortJSONOutput returns the container info in its raw, sorted form
func getAndSortJSONParams(containers []*libpod.Container, opts batchcontainer.PsOptions) ([]psJSONParams, error) {
	var (
		psOutput psSorted
		ns       *batchcontainer.Namespace
	)
	for _, ctr := range containers {
		batchInfo, err := batchcontainer.BatchContainerOp(ctr, opts)
		if err != nil {
			if errors.Cause(err) == libpod.ErrNoSuchCtr {
				logrus.Warn(err)
				continue
			}
			return nil, err
		}

		if opts.Namespace {
			ns = batchcontainer.GetNamespaces(batchInfo.Pid)
		}
		params := psJSONParams{
			ID:               ctr.ID(),
			Image:            batchInfo.ConConfig.RootfsImageName,
			ImageID:          batchInfo.ConConfig.RootfsImageID,
			Command:          batchInfo.ConConfig.Spec.Process.Args,
			ExitCode:         batchInfo.ExitCode,
			Exited:           batchInfo.Exited,
			CreatedAt:        batchInfo.ConConfig.CreatedTime,
			StartedAt:        batchInfo.StartedTime,
			ExitedAt:         batchInfo.ExitedTime,
			Status:           batchInfo.ConState.String(),
			PID:              batchInfo.Pid,
			Ports:            batchInfo.ConConfig.PortMappings,
			Size:             batchInfo.Size,
			Names:            batchInfo.ConConfig.Name,
			Labels:           batchInfo.ConConfig.Labels,
			Mounts:           batchInfo.ConConfig.UserVolumes,
			ContainerRunning: batchInfo.ConState == libpod.ContainerStateRunning,
			Namespaces:       ns,
			Pod:              ctr.PodID(),
		}

		psOutput = append(psOutput, params)
	}
	return sortPsOutput(opts.Sort, psOutput)
}

func generatePsOutput(containers []*libpod.Container, opts batchcontainer.PsOptions) error {
	if len(containers) == 0 && opts.Format != formats.JSONString {
		return nil
	}
	psOutput, err := getAndSortJSONParams(containers, opts)
	if err != nil {
		return err
	}
	var out formats.Writer

	switch opts.Format {
	case formats.JSONString:
		if err != nil {
			return errors.Wrapf(err, "unable to create JSON for output")
		}
		out = formats.JSONStructArray{Output: psToGeneric([]psTemplateParams{}, psOutput)}
	default:
		psOutput, err := getTemplateOutput(psOutput, opts)
		if err != nil {
			return errors.Wrapf(err, "unable to create output")
		}
		out = formats.StdoutTemplateArray{Output: psToGeneric(psOutput, []psJSONParams{}), Template: opts.Format, Fields: psOutput[0].headerMap()}
	}

	return formats.Writer(out).Out()
}

// getLabels converts the labels to a string of the form "key=value, key2=value2"
func formatLabels(labels map[string]string) string {
	var arr []string
	if len(labels) > 0 {
		for key, val := range labels {
			temp := key + "=" + val
			arr = append(arr, temp)
		}
		return strings.Join(arr, ",")
	}
	return ""
}

// getMounts converts the volumes mounted to a string of the form "mount1, mount2"
// it truncates it if noTrunc is false
func getMounts(mounts []string, noTrunc bool) string {
	return strings.Join(getMountsArray(mounts, noTrunc), ",")
}

func getMountsArray(mounts []string, noTrunc bool) []string {
	var arr []string
	if len(mounts) == 0 {
		return mounts
	}
	for _, mount := range mounts {
		splitArr := strings.Split(mount, ":")
		if len(splitArr[0]) > mountTruncLength && !noTrunc {
			arr = append(arr, splitArr[0][:mountTruncLength]+"...")
			continue
		}
		arr = append(arr, splitArr[0])
	}
	return arr
}

// portsToString converts the ports used to a string of the from "port1, port2"
func portsToString(ports []ocicni.PortMapping) string {
	var portDisplay []string
	if len(ports) == 0 {
		return ""
	}
	for _, v := range ports {
		hostIP := v.HostIP
		if hostIP == "" {
			hostIP = "0.0.0.0"
		}
		portDisplay = append(portDisplay, fmt.Sprintf("%s:%d->%d/%s", hostIP, v.HostPort, v.ContainerPort, v.Protocol))
	}
	return strings.Join(portDisplay, ", ")
}
