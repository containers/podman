package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/docker/go-units"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/cmd/podman/formats"
	"github.com/projectatomic/libpod/libpod"
	"github.com/projectatomic/libpod/pkg/util"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"k8s.io/apimachinery/pkg/fields"
)

const mountTruncLength = 12

type psOptions struct {
	all       bool
	filter    string
	format    string
	last      int
	latest    bool
	noTrunc   bool
	quiet     bool
	size      bool
	label     string
	namespace bool
}

type psTemplateParams struct {
	ID         string
	Image      string
	Command    string
	CreatedAt  string
	RunningFor string
	Status     string
	Ports      string
	Size       string
	Names      string
	Labels     string
	Mounts     string
	PID        int
	Cgroup     string
	IPC        string
	MNT        string
	NET        string
	PIDNS      string
	User       string
	UTS        string
}

// psJSONParams is only used when the JSON format is specified,
// and is better for data processing from JSON.
// psJSONParams will be populated by data from libpod.Container,
// the members of the struct are the sama data types as their sources.
type psJSONParams struct {
	ID               string               `json:"id"`
	Image            string               `json:"image"`
	ImageID          string               `json:"image_id"`
	Command          []string             `json:"command"`
	CreatedAt        time.Time            `json:"createdAt"`
	RunningFor       time.Duration        `json:"runningFor"`
	Status           string               `json:"status"`
	Ports            []ocicni.PortMapping `json:"ports"`
	RootFsSize       int64                `json:"rootFsSize"`
	RWSize           int64                `json:"rwSize"`
	Names            string               `json:"names"`
	Labels           fields.Set           `json:"labels"`
	Mounts           []specs.Mount        `json:"mounts"`
	ContainerRunning bool                 `json:"ctrRunning"`
	Namespaces       *namespace           `json:"namespace,omitempty"`
}

type namespace struct {
	PID    string `json:"pid,omitempty"`
	Cgroup string `json:"cgroup,omitempty"`
	IPC    string `json:"ipc,omitempty"`
	MNT    string `json:"mnt,omitempty"`
	NET    string `json:"net,omitempty"`
	PIDNS  string `json:"pidns,omitempty"`
	User   string `json:"user,omitempty"`
	UTS    string `json:"uts,omitempty"`
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
			Name:  "no-trunc",
			Usage: "Display the extended information",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Print the numeric IDs of the containers only",
		},
		cli.BoolFlag{
			Name:  "size, s",
			Usage: "Display the total file sizes",
		},
		cli.BoolFlag{
			Name:  "namespace, ns",
			Usage: "Display namespace information",
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
)

func psCmd(c *cli.Context) error {
	if err := validateFlags(c, psFlags); err != nil {
		return err
	}

	if err := checkFlagsPassed(c); err != nil {
		return errors.Wrapf(err, "error with flags passed")
	}

	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}

	defer runtime.Shutdown(false)

	if len(c.Args()) > 0 {
		return errors.Errorf("too many arguments, ps takes no arguments")
	}

	format := genPsFormat(c.String("format"), c.Bool("quiet"), c.Bool("size"), c.Bool("namespace"))

	opts := psOptions{
		all:       c.Bool("all"),
		filter:    c.String("filter"),
		format:    format,
		last:      c.Int("last"),
		latest:    c.Bool("latest"),
		noTrunc:   c.Bool("no-trunc"),
		quiet:     c.Bool("quiet"),
		size:      c.Bool("size"),
		namespace: c.Bool("namespace"),
	}

	var filterFuncs []libpod.ContainerFilter
	// When we are dealing with latest or last=n, we need to
	// get all containers.
	if !opts.all && !opts.latest && opts.last < 1 {
		// only get running containers
		filterFuncs = append(filterFuncs, func(c *libpod.Container) bool {
			state, _ := c.State()
			return state == libpod.ContainerStateRunning
		})
	}

	if opts.filter != "" {
		filters := strings.Split(opts.filter, ",")
		for _, f := range filters {
			filterSplit := strings.Split(f, "=")
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

	containers, err := runtime.GetContainers(filterFuncs...)
	var outputContainers []*libpod.Container
	if opts.latest && len(containers) > 0 {
		outputContainers = append(outputContainers, containers[0])
	} else if opts.last > 0 && opts.last <= len(containers) {
		outputContainers = append(outputContainers, containers[:opts.last]...)
	} else {
		outputContainers = containers
	}

	return generatePsOutput(outputContainers, opts)
}

// checkFlagsPassed checks if mutually exclusive flags are passed together
func checkFlagsPassed(c *cli.Context) error {
	// latest, and last are mutually exclusive.
	if c.Int("last") >= 0 && c.Bool("latest") {
		return errors.Errorf("last and latest are mutually exclusive")
	}
	// quiet, size, namespace, and format with Go template are mutually exclusive
	flags := 0
	if c.Bool("quiet") {
		flags++
	}
	if c.Bool("size") {
		flags++
	}
	if c.Bool("namespace") {
		flags++
	}
	if c.IsSet("format") && c.String("format") != formats.JSONString {
		flags++
	}
	if flags > 1 {
		return errors.Errorf("quiet, size, namespace, and format with Go template are mutually exclusive")
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
		return func(c *libpod.Container) bool {
			for _, label := range c.Labels() {
				if label == filterValue {
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
			ec, err := c.ExitCode()
			if ec == int32(exitCode) && err == nil {
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
func genPsFormat(format string, quiet, size, namespace bool) string {
	if format != "" {
		// "\t" from the command line is not being recognized as a tab
		// replacing the string "\t" to a tab character if the user passes in "\t"
		return strings.Replace(format, `\t`, "\t", -1)
	}
	if quiet {
		return formats.IDString
	}
	if namespace {
		return "table {{.ID}}\t{{.Names}}\t{{.PID}}\t{{.Cgroup}}\t{{.IPC}}\t{{.MNT}}\t{{.NET}}\t{{.PIDNS}}\t{{.User}}\t{{.UTS}}\t"
	}
	format = "table {{.ID}}\t{{.Image}}\t{{.Command}}\t{{.CreatedAt}}\t{{.Status}}\t{{.Ports}}\t{{.Names}}\t"
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

// getTemplateOutput returns the modified container information
func getTemplateOutput(containers []*libpod.Container, opts psOptions) ([]psTemplateParams, error) {
	var (
		psOutput     []psTemplateParams
		status, size string
		ns           *namespace
	)

	for _, ctr := range containers {
		batchInfo, err := batchContainerOp(ctr, opts)
		if err != nil {
			// If the error was ErrNoSuchCtr, it was probably
			// removed sometime after we got the initial list.
			// Just ignore it.
			if errors.Cause(err) == libpod.ErrNoSuchCtr {
				logrus.Debugf("Container %s removed before batch, ignoring in output", ctr.ID())
				continue
			}

			return nil, err
		}
		ctrID := ctr.ID()
		runningFor := ""
		// If the container has not be started, the "zero" value of time is 0001-01-01 00:00:00 +0000 UTC
		// which would make the time elapsed about a few hundred of years. So checking for the "zero" value of time.Time
		if batchInfo.startedTime != (time.Time{}) {
			runningFor = units.HumanDuration(time.Since(batchInfo.startedTime))
		}
		createdAt := batchInfo.conConfig.CreatedTime.Format("2006-01-02 15:04:05 -0700 MST")
		imageName := batchInfo.conConfig.RootfsImageName

		var createArtifact createConfig
		artifact, err := ctr.GetArtifact("create-config")
		if err == nil {
			if err := json.Unmarshal(artifact, &createArtifact); err != nil {
				return nil, err
			}
		} else {
			logrus.Errorf("couldn't get some ps information, error getting artifact %q: %v", ctr.ID(), err)
		}
		if opts.namespace {
			ns = getNamespaces(batchInfo.pid)
		}
		if opts.size {

			size = units.HumanSizeWithPrecision(float64(batchInfo.rwSize), 3) + " (virtual " + units.HumanSizeWithPrecision(float64(batchInfo.rootFsSize), 3) + ")"
		}

		command := strings.Join(batchInfo.conConfig.Spec.Process.Args, " ")
		if !opts.noTrunc {
			if len(command) > 20 {
				command = command[:19] + "..."
			}
		}
		ports := portsToString(batchInfo.conConfig.PortMappings)
		mounts := getMounts(createArtifact.Volumes, opts.noTrunc)
		labels := formatLabels(ctr.Labels())

		switch batchInfo.conState {
		case libpod.ContainerStateStopped:
			status = fmt.Sprintf("Exited (%d) %s ago", batchInfo.exitCode, runningFor)
		case libpod.ContainerStateRunning:
			status = "Up " + runningFor + " ago"
		case libpod.ContainerStatePaused:
			status = "Paused"
		case libpod.ContainerStateCreated, libpod.ContainerStateConfigured:
			status = "Created"
		default:
			status = "Dead"
		}

		if !opts.noTrunc {
			ctrID = shortID(ctr.ID())
			imageName = batchInfo.conConfig.RootfsImageName
		}

		params := psTemplateParams{
			ID:         ctrID,
			Image:      imageName,
			Command:    command,
			CreatedAt:  createdAt,
			RunningFor: runningFor,
			Status:     status,
			Ports:      ports,
			Size:       size,
			Names:      ctr.Name(),
			Labels:     labels,
			Mounts:     mounts,
			PID:        batchInfo.pid,
		}

		if opts.namespace {
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

func getNamespaces(pid int) *namespace {
	ctrPID := strconv.Itoa(pid)
	cgroup, _ := getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "cgroup"))
	ipc, _ := getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "ipc"))
	mnt, _ := getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "mnt"))
	net, _ := getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "net"))
	pidns, _ := getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "pid"))
	user, _ := getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "user"))
	uts, _ := getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "uts"))

	return &namespace{
		PID:    ctrPID,
		Cgroup: cgroup,
		IPC:    ipc,
		MNT:    mnt,
		NET:    net,
		PIDNS:  pidns,
		User:   user,
		UTS:    uts,
	}
}

func getNamespaceInfo(path string) (string, error) {
	val, err := os.Readlink(path)
	if err != nil {
		return "", errors.Wrapf(err, "error getting info from %q", path)
	}
	return getStrFromSquareBrackets(val), nil
}

// getJSONOutput returns the container info in its raw form
func getJSONOutput(containers []*libpod.Container, opts psOptions) ([]psJSONParams, error) {
	var (
		psOutput []psJSONParams
		ns       *namespace
	)
	for _, ctr := range containers {
		batchInfo, err := batchContainerOp(ctr, opts)
		if err != nil {
			return nil, err
		}
		if opts.namespace {
			ns = getNamespaces(batchInfo.pid)
		}
		params := psJSONParams{
			ID:               ctr.ID(),
			Image:            batchInfo.conConfig.RootfsImageName,
			ImageID:          batchInfo.conConfig.RootfsImageID,
			Command:          batchInfo.conConfig.Spec.Process.Args,
			CreatedAt:        batchInfo.conConfig.CreatedTime,
			RunningFor:       time.Since(batchInfo.conConfig.CreatedTime),
			Status:           batchInfo.conState.String(),
			Ports:            batchInfo.conConfig.PortMappings,
			RootFsSize:       batchInfo.rootFsSize,
			RWSize:           batchInfo.rwSize,
			Names:            batchInfo.conConfig.Name,
			Labels:           batchInfo.conConfig.Labels,
			Mounts:           batchInfo.conConfig.Spec.Mounts,
			ContainerRunning: batchInfo.conState == libpod.ContainerStateRunning,
			Namespaces:       ns,
		}
		psOutput = append(psOutput, params)
	}
	return psOutput, nil
}

func generatePsOutput(containers []*libpod.Container, opts psOptions) error {
	if len(containers) == 0 && opts.format != formats.JSONString {
		return nil
	}
	var out formats.Writer

	switch opts.format {
	case formats.JSONString:
		psOutput, err := getJSONOutput(containers, opts)
		if err != nil {
			return errors.Wrapf(err, "unable to create JSON for output")
		}
		out = formats.JSONStructArray{Output: psToGeneric([]psTemplateParams{}, psOutput)}
	default:
		psOutput, err := getTemplateOutput(containers, opts)
		if err != nil {
			return errors.Wrapf(err, "unable to create output")
		}
		out = formats.StdoutTemplateArray{Output: psToGeneric(psOutput, []psJSONParams{}), Template: opts.format, Fields: psOutput[0].headerMap()}
	}

	return formats.Writer(out).Out()
}

// getStrFromSquareBrackets gets the string inside [] from a string
func getStrFromSquareBrackets(cmd string) string {
	reg, err := regexp.Compile(".*\\[|\\].*")
	if err != nil {
		return ""
	}
	arr := strings.Split(reg.ReplaceAllLiteralString(cmd, ""), ",")
	return strings.Join(arr, ",")
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
	var arr []string
	if len(mounts) == 0 {
		return ""
	}
	for _, mount := range mounts {
		splitArr := strings.Split(mount, ":")
		if len(splitArr[0]) > mountTruncLength && !noTrunc {
			arr = append(arr, splitArr[0][:mountTruncLength]+"...")
			continue
		}
		arr = append(arr, splitArr[0])
	}
	return strings.Join(arr, ",")
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

type batchContainerStruct struct {
	conConfig          *libpod.ContainerConfig
	conState           libpod.ContainerStatus
	exitCode           int32
	pid                int
	rootFsSize, rwSize int64
	startedTime        time.Time
}

func batchContainerOp(ctr *libpod.Container, opts psOptions) (batchContainerStruct, error) {
	var (
		conConfig          *libpod.ContainerConfig
		conState           libpod.ContainerStatus
		err                error
		exitCode           int32
		pid                int
		rootFsSize, rwSize int64
		startedTime        time.Time
	)

	batchErr := ctr.Batch(func(c *libpod.Container) error {
		conConfig = c.Config()
		conState, err = c.State()
		if err != nil {
			return errors.Wrapf(err, "unable to obtain container state")
		}

		exitCode, err = c.ExitCode()
		if err != nil {
			return errors.Wrapf(err, "unable to obtain container exit code")
		}
		startedTime, err = c.StartedTime()
		if err != nil {
			logrus.Errorf("error getting started time for %q: %v", c.ID(), err)
		}

		if !opts.size && !opts.namespace {
			return nil
		}

		if opts.namespace {
			pid, err = c.PID()
			if err != nil {
				return errors.Wrapf(err, "unable to obtain container pid")
			}
		}
		if opts.size {
			rootFsSize, err = c.RootFsSize()
			if err != nil {
				logrus.Errorf("error getting root fs size for %q: %v", c.ID(), err)
			}

			rwSize, err = c.RWSize()
			if err != nil {
				logrus.Errorf("error getting rw size for %q: %v", c.ID(), err)
			}

		}
		return nil
	})
	if batchErr != nil {
		return batchContainerStruct{}, batchErr
	}
	return batchContainerStruct{
		conConfig:   conConfig,
		conState:    conState,
		exitCode:    exitCode,
		pid:         pid,
		rootFsSize:  rootFsSize,
		rwSize:      rwSize,
		startedTime: startedTime,
	}, nil
}
