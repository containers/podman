package main

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/docker/go-units"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/fields"

	"github.com/kubernetes-incubator/cri-o/cmd/kpod/formats"
	"github.com/kubernetes-incubator/cri-o/libkpod"
	"github.com/kubernetes-incubator/cri-o/oci"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

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
// psJSONParams will be populated by data from libkpod.ContainerData,
// the members of the struct are the sama data types as their sources.
type psJSONParams struct {
	ID               string              `json:"id"`
	Image            string              `json:"image"`
	ImageID          string              `json:"image_id"`
	Command          string              `json:"command"`
	CreatedAt        time.Time           `json:"createdAt"`
	RunningFor       time.Duration       `json:"runningFor"`
	Status           string              `json:"status"`
	Ports            map[string]struct{} `json:"ports"`
	Size             uint                `json:"size"`
	Names            string              `json:"names"`
	Labels           fields.Set          `json:"labels"`
	Mounts           []specs.Mount       `json:"mounts"`
	ContainerRunning bool                `json:"ctrRunning"`
	Namespaces       *namespace          `json:"namespace,omitempty"`
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
		Name:        "ps",
		Usage:       "List containers",
		Description: psDescription,
		Flags:       psFlags,
		Action:      psCmd,
		ArgsUsage:   "",
	}
)

func psCmd(c *cli.Context) error {
	if err := validateFlags(c, psFlags); err != nil {
		return err
	}
	config, err := getConfig(c)
	if err != nil {
		return errors.Wrapf(err, "could not get config")
	}
	server, err := libkpod.New(config)
	if err != nil {
		return errors.Wrapf(err, "error creating server")
	}
	if err := server.Update(); err != nil {
		return errors.Wrapf(err, "error updating list of containers")
	}

	if len(c.Args()) > 0 {
		return errors.Errorf("too many arguments, ps takes no arguments")
	}

	format := genPsFormat(c.Bool("quiet"), c.Bool("size"), c.Bool("namespace"))
	if c.IsSet("format") {
		format = c.String("format")
	}

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

	// all, latest, and last are mutually exclusive. Only one flag can be used at a time
	exclusiveOpts := 0
	if opts.last >= 0 {
		exclusiveOpts++
	}
	if opts.latest {
		exclusiveOpts++
	}
	if opts.all {
		exclusiveOpts++
	}
	if exclusiveOpts > 1 {
		return errors.Errorf("Last, latest and all are mutually exclusive")
	}

	containers, err := server.ListContainers()
	if err != nil {
		return errors.Wrapf(err, "error getting containers from server")
	}
	var params *FilterParamsPS
	if opts.filter != "" {
		params, err = parseFilter(opts.filter, containers)
		if err != nil {
			return errors.Wrapf(err, "error parsing filter")
		}
	} else {
		params = nil
	}

	containerList := getContainersMatchingFilter(containers, params, server)

	return generatePsOutput(containerList, server, opts)
}

// generate the template based on conditions given
func genPsFormat(quiet, size, namespace bool) (format string) {
	if quiet {
		return formats.IDString
	}
	if namespace {
		format = "table {{.ID}}\t{{.Names}}\t{{.PID}}\t{{.Cgroup}}\t{{.IPC}}\t{{.MNT}}\t{{.NET}}\t{{.PIDNS}}\t{{.User}}\t{{.UTS}}\t"
		return
	}
	format = "table {{.ID}}\t{{.Image}}\t{{.Command}}\t{{.CreatedAt}}\t{{.Status}}\t{{.Ports}}\t{{.Names}}\t"
	if size {
		format += "{{.Size}}\t"
	}
	return
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

// getContainers gets the containers that match the flags given
func getContainers(containers []*libkpod.ContainerData, opts psOptions) []*libkpod.ContainerData {
	var containersOutput []*libkpod.ContainerData
	if opts.last >= 0 && opts.last < len(containers) {
		for i := 0; i < opts.last; i++ {
			containersOutput = append(containersOutput, containers[i])
		}
		return containersOutput
	}
	if opts.latest {
		return []*libkpod.ContainerData{containers[0]}
	}
	if opts.all || opts.last >= len(containers) {
		return containers
	}
	for _, ctr := range containers {
		if ctr.State.Status == oci.ContainerStateRunning {
			containersOutput = append(containersOutput, ctr)
		}
	}
	return containersOutput
}

// getTemplateOutput returns the modified container information
func getTemplateOutput(containers []*libkpod.ContainerData, opts psOptions) (psOutput []psTemplateParams) {
	var status string
	for _, ctr := range containers {
		ctrID := ctr.ID
		runningFor := units.HumanDuration(time.Since(ctr.State.Created))
		createdAt := runningFor + " ago"
		command := getStrFromSquareBrackets(ctr.ImageCreatedBy)
		imageName := ctr.FromImage
		mounts := getMounts(ctr.Mounts, opts.noTrunc)
		ports := getPorts(ctr.Config.ExposedPorts)
		size := units.HumanSize(float64(ctr.SizeRootFs))
		labels := getLabels(ctr.Labels)

		ns := getNamespaces(ctr.State.Pid)

		switch ctr.State.Status {
		case oci.ContainerStateStopped:
			status = "Exited (" + strconv.FormatInt(int64(ctr.State.ExitCode), 10) + ") " + runningFor + " ago"
		case oci.ContainerStateRunning:
			status = "Up " + runningFor + " ago"
		case oci.ContainerStatePaused:
			status = "Paused"
		default:
			status = "Created"
		}

		if !opts.noTrunc {
			ctrID = ctr.ID[:idTruncLength]
			imageName = getImageName(ctr.FromImage)
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
			Names:      ctr.Name,
			Labels:     labels,
			Mounts:     mounts,
			PID:        ctr.State.Pid,
			Cgroup:     ns.Cgroup,
			IPC:        ns.IPC,
			MNT:        ns.MNT,
			NET:        ns.NET,
			PIDNS:      ns.PID,
			User:       ns.User,
			UTS:        ns.UTS,
		}
		psOutput = append(psOutput, params)
	}
	return
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
func getJSONOutput(containers []*libkpod.ContainerData, nSpace bool) (psOutput []psJSONParams) {
	var ns *namespace
	for _, ctr := range containers {
		if nSpace {
			ns = getNamespaces(ctr.State.Pid)
		}

		params := psJSONParams{
			ID:               ctr.ID,
			Image:            ctr.FromImage,
			ImageID:          ctr.FromImageID,
			Command:          getStrFromSquareBrackets(ctr.ImageCreatedBy),
			CreatedAt:        ctr.State.Created,
			RunningFor:       time.Since(ctr.State.Created),
			Status:           ctr.State.Status,
			Ports:            ctr.Config.ExposedPorts,
			Size:             ctr.SizeRootFs,
			Names:            ctr.Name,
			Labels:           ctr.Labels,
			Mounts:           ctr.Mounts,
			ContainerRunning: ctr.State.Status == oci.ContainerStateRunning,
			Namespaces:       ns,
		}
		psOutput = append(psOutput, params)
	}
	return
}

func generatePsOutput(containers []*libkpod.ContainerData, server *libkpod.ContainerServer, opts psOptions) error {
	containersOutput := getContainers(containers, opts)
	// In the case of JSON, we want to continue so we at least pass
	// {} --valid JSON-- to the consumer
	if len(containersOutput) == 0 && opts.format != formats.JSONString {
		return nil
	}

	var out formats.Writer

	switch opts.format {
	case formats.JSONString:
		psOutput := getJSONOutput(containersOutput, opts.namespace)
		out = formats.JSONStructArray{Output: psToGeneric([]psTemplateParams{}, psOutput)}
	default:
		psOutput := getTemplateOutput(containersOutput, opts)
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

// getImageName shortens the image name
func getImageName(img string) string {
	arr := strings.Split(img, "/")
	if arr[0] == "docker.io" && arr[1] == "library" {
		img = strings.Join(arr[2:], "/")
	} else if arr[0] == "docker.io" {
		img = strings.Join(arr[1:], "/")
	}
	return img
}

// getLabels converts the labels to a string of the form "key=value, key2=value2"
func getLabels(labels fields.Set) string {
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
func getMounts(mounts []specs.Mount, noTrunc bool) string {
	var arr []string
	if len(mounts) == 0 {
		return ""
	}
	for _, mount := range mounts {
		if noTrunc {
			arr = append(arr, mount.Source)
			continue
		}
		tempArr := strings.SplitAfter(mount.Source, "/")
		if len(tempArr) >= 3 {
			arr = append(arr, strings.Join(tempArr[:3], ""))
		} else {
			arr = append(arr, mount.Source)
		}
	}
	return strings.Join(arr, ",")
}

// getPorts converts the ports used to a string of the from "port1, port2"
func getPorts(ports map[string]struct{}) string {
	var arr []string
	if len(ports) == 0 {
		return ""
	}
	for key := range ports {
		arr = append(arr, key)
	}
	return strings.Join(arr, ",")
}

// FilterParamsPS contains the filter options for ps
type FilterParamsPS struct {
	id       string
	label    string
	name     string
	exited   int32
	status   string
	ancestor string
	before   time.Time
	since    time.Time
	volume   string
}

// parseFilter takes a filter string and a list of containers and filters it
func parseFilter(filter string, containers []*oci.Container) (*FilterParamsPS, error) {
	params := new(FilterParamsPS)
	allFilters := strings.Split(filter, ",")

	for _, param := range allFilters {
		pair := strings.SplitN(param, "=", 2)
		switch strings.TrimSpace(pair[0]) {
		case "id":
			params.id = pair[1]
		case "label":
			params.label = pair[1]
		case "name":
			params.name = pair[1]
		case "exited":
			exitedCode, err := strconv.ParseInt(pair[1], 10, 32)
			if err != nil {
				return nil, errors.Errorf("exited code out of range %q", pair[1])
			}
			params.exited = int32(exitedCode)
		case "status":
			params.status = pair[1]
		case "ancestor":
			params.ancestor = pair[1]
		case "before":
			if ctr, err := findContainer(containers, pair[1]); err == nil {
				params.before = ctr.CreatedAt()
			} else {
				return nil, errors.Wrapf(err, "no such container %q", pair[1])
			}
		case "since":
			if ctr, err := findContainer(containers, pair[1]); err == nil {
				params.before = ctr.CreatedAt()
			} else {
				return nil, errors.Wrapf(err, "no such container %q", pair[1])
			}
		case "volume":
			params.volume = pair[1]
		default:
			return nil, errors.Errorf("invalid filter %q", pair[0])
		}
	}
	return params, nil
}

// findContainer finds a container with a specific name or id from a list of containers
func findContainer(containers []*oci.Container, ref string) (*oci.Container, error) {
	for _, ctr := range containers {
		if strings.HasPrefix(ctr.ID(), ref) || ctr.Name() == ref {
			return ctr, nil
		}
	}
	return nil, errors.Errorf("could not find container")
}

// matchesFilter checks if a container matches all the filter parameters
func matchesFilter(ctrData *libkpod.ContainerData, params *FilterParamsPS) bool {
	if params == nil {
		return true
	}
	if params.id != "" && !matchesID(ctrData, params.id) {
		return false
	}
	if params.name != "" && !matchesName(ctrData, params.name) {
		return false
	}
	if !params.before.IsZero() && !matchesBeforeContainer(ctrData, params.before) {
		return false
	}
	if !params.since.IsZero() && !matchesSinceContainer(ctrData, params.since) {
		return false
	}
	if params.exited > 0 && !matchesExited(ctrData, params.exited) {
		return false
	}
	if params.status != "" && !matchesStatus(ctrData, params.status) {
		return false
	}
	if params.ancestor != "" && !matchesAncestor(ctrData, params.ancestor) {
		return false
	}
	if params.label != "" && !matchesLabel(ctrData, params.label) {
		return false
	}
	if params.volume != "" && !matchesVolume(ctrData, params.volume) {
		return false
	}
	return true
}

// GetContainersMatchingFilter returns a slice of all the containers that match the provided filter parameters
func getContainersMatchingFilter(containers []*oci.Container, filter *FilterParamsPS, server *libkpod.ContainerServer) []*libkpod.ContainerData {
	var filteredCtrs []*libkpod.ContainerData
	for _, ctr := range containers {
		ctrData, err := server.GetContainerData(ctr.ID(), true)
		if err != nil {
			logrus.Warn("unable to get container data for matched container")
		}
		if filter == nil || matchesFilter(ctrData, filter) {
			filteredCtrs = append(filteredCtrs, ctrData)
		}
	}
	return filteredCtrs
}

// matchesID returns true if the id's match
func matchesID(ctrData *libkpod.ContainerData, id string) bool {
	return strings.HasPrefix(ctrData.ID, id)
}

// matchesBeforeContainer returns true if the container was created before the filter image
func matchesBeforeContainer(ctrData *libkpod.ContainerData, beforeTime time.Time) bool {
	return ctrData.State.Created.Before(beforeTime)
}

// matchesSincecontainer returns true if the container was created since the filter image
func matchesSinceContainer(ctrData *libkpod.ContainerData, sinceTime time.Time) bool {
	return ctrData.State.Created.After(sinceTime)
}

// matchesLabel returns true if the container label matches that of the filter label
func matchesLabel(ctrData *libkpod.ContainerData, label string) bool {
	pair := strings.SplitN(label, "=", 2)
	if val, ok := ctrData.Labels[pair[0]]; ok {
		if len(pair) == 2 && val == pair[1] {
			return true
		}
		if len(pair) == 1 {
			return true
		}
		return false
	}
	return false
}

// matchesName returns true if the names are identical
func matchesName(ctrData *libkpod.ContainerData, name string) bool {
	return ctrData.Name == name
}

// matchesExited returns true if the exit codes are identical
func matchesExited(ctrData *libkpod.ContainerData, exited int32) bool {
	return ctrData.State.ExitCode == exited
}

// matchesStatus returns true if the container status matches that of filter status
func matchesStatus(ctrData *libkpod.ContainerData, status string) bool {
	return ctrData.State.Status == status
}

// matchesAncestor returns true if filter ancestor is in container image name
func matchesAncestor(ctrData *libkpod.ContainerData, ancestor string) bool {
	return strings.Contains(ctrData.FromImage, ancestor)
}

// matchesVolue returns true if the volume mounted or path to volue of the container matches that of filter volume
func matchesVolume(ctrData *libkpod.ContainerData, volume string) bool {
	for _, vol := range ctrData.Mounts {
		if strings.Contains(vol.Source, volume) {
			return true
		}
	}
	return false
}
