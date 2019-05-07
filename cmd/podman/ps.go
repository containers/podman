package main

import (
	"fmt"
	"html/template"
	"os"
	"reflect"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	tm "github.com/buger/goterm"
	"github.com/containers/buildah/pkg/formats"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/fields"
)

const (
	mountTruncLength = 12
	hid              = "CONTAINER ID"
	himage           = "IMAGE"
	hcommand         = "COMMAND"
	hcreated         = "CREATED"
	hstatus          = "STATUS"
	hports           = "PORTS"
	hnames           = "NAMES"
	hsize            = "SIZE"
	hinfra           = "IS INFRA"
	hpod             = "POD"
	nspid            = "PID"
	nscgroup         = "CGROUPNS"
	nsipc            = "IPC"
	nsmnt            = "MNT"
	nsnet            = "NET"
	nspidns          = "PIDNS"
	nsuserns         = "USERNS"
	nsuts            = "UTS"
)

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
	CGROUPNS      string
	IPC           string
	MNT           string
	NET           string
	PIDNS         string
	USERNS        string
	UTS           string
	Pod           string
	IsInfra       bool
}

// psJSONParams is used as a base structure for the psParams
// If template output is requested, psJSONParams will be converted to
// psTemplateParams.
// psJSONParams will be populated by data from libpod.Container,
// the members of the struct are the sama data types as their sources.
type psJSONParams struct {
	ID               string                `json:"id"`
	Image            string                `json:"image"`
	ImageID          string                `json:"image_id"`
	Command          []string              `json:"command"`
	ExitCode         int32                 `json:"exitCode"`
	Exited           bool                  `json:"exited"`
	CreatedAt        time.Time             `json:"createdAt"`
	StartedAt        time.Time             `json:"startedAt"`
	ExitedAt         time.Time             `json:"exitedAt"`
	Status           string                `json:"status"`
	PID              int                   `json:"PID"`
	Ports            []ocicni.PortMapping  `json:"ports"`
	Size             *shared.ContainerSize `json:"size,omitempty"`
	Names            string                `json:"names"`
	Labels           fields.Set            `json:"labels"`
	Mounts           []string              `json:"mounts"`
	ContainerRunning bool                  `json:"ctrRunning"`
	Namespaces       *shared.Namespace     `json:"namespace,omitempty"`
	Pod              string                `json:"pod,omitempty"`
	IsInfra          bool                  `json:"infra"`
}

// Type declaration and functions for sorting the PS output
type psSorted []shared.PsContainerOutput

func (a psSorted) Len() int      { return len(a) }
func (a psSorted) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

type psSortedCommand struct{ psSorted }

func (a psSortedCommand) Less(i, j int) bool {
	return a.psSorted[i].Command < a.psSorted[j].Command
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
	psCommand     cliconfig.PsValues
	psDescription = "Prints out information about the containers"
	_psCommand    = cobra.Command{
		Use:   "ps",
		Args:  noSubArgs,
		Short: "List containers",
		Long:  psDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			psCommand.InputArgs = args
			psCommand.GlobalFlags = MainGlobalOpts
			psCommand.Remote = remoteclient
			return psCmd(&psCommand)
		},
		Example: `podman ps -a
  podman ps -a --format "{{.ID}}  {{.Image}}  {{.Labels}}  {{.Mounts}}"
  podman ps --size --sort names`,
	}
)

func psInit(command *cliconfig.PsValues) {
	command.SetHelpTemplate(HelpTemplate())
	command.SetUsageTemplate(UsageTemplate())
	flags := command.Flags()
	flags.BoolVarP(&command.All, "all", "a", false, "Show all the containers, default is only running containers")
	flags.StringSliceVarP(&command.Filter, "filter", "f", []string{}, "Filter output based on conditions given")
	flags.StringVar(&command.Format, "format", "", "Pretty-print containers to JSON or using a Go template")
	flags.IntVarP(&command.Last, "last", "n", -1, "Print the n last created containers (all states)")
	flags.BoolVarP(&command.Latest, "latest", "l", false, "Show the latest container created (all states)")
	flags.BoolVar(&command.Namespace, "namespace", false, "Display namespace information")
	flags.BoolVar(&command.Namespace, "ns", false, "Display namespace information")
	flags.BoolVar(&command.NoTrunct, "no-trunc", false, "Display the extended information")
	flags.BoolVarP(&command.Pod, "pod", "p", false, "Print the ID and name of the pod the containers are associated with")
	flags.BoolVarP(&command.Quiet, "quiet", "q", false, "Print the numeric IDs of the containers only")
	flags.BoolVarP(&command.Size, "size", "s", false, "Display the total file sizes")
	flags.StringVar(&command.Sort, "sort", "created", "Sort output by command, created, id, image, names, runningfor, size, or status")
	flags.BoolVar(&command.Sync, "sync", false, "Sync container state with OCI runtime")
	flags.UintVarP(&command.Watch, "watch", "w", 0, "Watch the ps output on an interval in seconds")

	markFlagHiddenForRemoteClient("latest", flags)
}

func init() {
	psCommand.Command = &_psCommand
	psInit(&psCommand)
}

func psCmd(c *cliconfig.PsValues) error {
	var watch bool

	if c.Watch > 0 {
		watch = true
	}

	if c.Watch > 0 && c.Latest {
		return errors.New("the watch and latest flags cannot be used together")
	}

	if err := checkFlagsPassed(c); err != nil {
		return errors.Wrapf(err, "error with flags passed")
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}

	defer runtime.Shutdown(false)

	if !watch {
		if err := psDisplay(c, runtime); err != nil {
			return err
		}
	} else {
		for {
			tm.Clear()
			tm.MoveCursor(1, 1)
			tm.Flush()
			if err := psDisplay(c, runtime); err != nil {
				return err
			}
			time.Sleep(time.Duration(c.Watch) * time.Second)
			tm.Clear()
			tm.MoveCursor(1, 1)
			tm.Flush()
		}
	}
	return nil
}

func printQuiet(containers []shared.PsContainerOutput) error {
	for _, c := range containers {
		fmt.Println(c.ID)
	}
	return nil
}

// checkFlagsPassed checks if mutually exclusive flags are passed together
func checkFlagsPassed(c *cliconfig.PsValues) error {
	// latest, and last are mutually exclusive.
	if c.Last >= 0 && c.Latest {
		return errors.Errorf("last and latest are mutually exclusive")
	}
	// Quiet conflicts with size, namespace, and format with a Go template
	if c.Quiet {
		if c.Size || c.Namespace || (c.Flag("format").Changed &&
			c.Format != formats.JSONString) {
			return errors.Errorf("quiet conflicts with size, namespace, and format with go template")
		}
	}
	// Size and namespace conflict with each other
	if c.Size && c.Namespace {
		return errors.Errorf("size and namespace options conflict")
	}
	return nil
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

func printFormat(format string, containers []shared.PsContainerOutput) error {
	// return immediately if no containers are present
	if len(containers) == 0 {
		return nil
	}

	// Use a tabwriter to align column format
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	// Make a map of the field names for the headers
	headerNames := make(map[string]string)
	v := reflect.ValueOf(containers[0])
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		headerNames[t.Field(i).Name] = t.Field(i).Name
	}

	// Spit out the header if "table" is present in the format
	if strings.HasPrefix(format, "table") {
		hformat := strings.Replace(strings.TrimSpace(format[5:]), " ", "\t", -1)
		format = hformat
		headerTmpl, err := template.New("header").Parse(hformat)
		if err != nil {
			return err
		}
		if err := headerTmpl.Execute(w, headerNames); err != nil {
			return err
		}
		fmt.Fprintln(w, "")
	}

	// Spit out the data rows now
	dataTmpl, err := template.New("data").Parse(format)
	if err != nil {
		return err
	}

	for _, container := range containers {
		if err := dataTmpl.Execute(w, container); err != nil {
			return err
		}
		fmt.Fprintln(w, "")
	}
	// Flush the writer
	return w.Flush()
}

func dumpJSON(containers []shared.PsContainerOutput) error {
	b, err := json.MarshalIndent(containers, "", "     ")
	if err != nil {
		return err
	}
	os.Stdout.Write(b)
	return nil
}

func psDisplay(c *cliconfig.PsValues, runtime *adapter.LocalRuntime) error {
	var (
		err error
	)
	opts := shared.PsOptions{
		All:       c.All,
		Format:    c.Format,
		Last:      c.Last,
		Latest:    c.Latest,
		NoTrunc:   c.NoTrunct,
		Pod:       c.Pod,
		Quiet:     c.Quiet,
		Size:      c.Size,
		Namespace: c.Namespace,
		Sort:      c.Sort,
		Sync:      c.Sync,
	}

	pss, err := runtime.Ps(c, opts)
	// Here and down
	if opts.Sort != "" {
		pss, err = sortPsOutput(opts.Sort, pss)
		if err != nil {
			return err
		}
	}

	// If quiet, print only cids and return
	if opts.Quiet {
		return printQuiet(pss)
	}

	// If the user wants their own GO template format
	if opts.Format != "" {
		if opts.Format == "json" {
			return dumpJSON(pss)
		}
		return printFormat(opts.Format, pss)
	}

	// Define a tab writer with stdout as the output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Output standard PS headers
	if !opts.Namespace {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s", hid, himage, hcommand, hcreated, hstatus, hports, hnames)
		// User wants pod info
		if opts.Pod {
			fmt.Fprintf(w, "\t%s", hpod)
		}
		//User wants size info
		if opts.Size {
			fmt.Fprintf(w, "\t%s", hsize)
		}
	} else {
		// Output Namespace headers
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s", hid, hnames, nspid, nscgroup, nsipc, nsmnt, nsnet, nspidns, nsuserns, nsuts)
	}

	// Now iterate each container and output its information
	for _, container := range pss {

		// Standard PS output
		if !opts.Namespace {
			fmt.Fprintf(w, "\n%s\t%s\t%s\t%s\t%s\t%s\t%s", container.ID, container.Image, container.Command, container.Created, container.Status, container.Ports, container.Names)
			// User wants pod info
			if opts.Pod {
				fmt.Fprintf(w, "\t%s", container.Pod)
			}
			//User wants size info
			if opts.Size {
				var size string
				if container.Size == nil {
					size = units.HumanSizeWithPrecision(0, 0)
				} else {
					size = units.HumanSizeWithPrecision(float64(container.Size.RwSize), 3) + " (virtual " + units.HumanSizeWithPrecision(float64(container.Size.RootFsSize), 3) + ")"
					fmt.Fprintf(w, "\t%s", size)
				}
			}

		} else {
			// Print namespace information
			ns := runtime.GetNamespaces(container)
			fmt.Fprintf(w, "\n%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s", container.ID, container.Names, container.Pid, ns.Cgroup, ns.IPC, ns.MNT, ns.NET, ns.PIDNS, ns.User, ns.UTS)
		}

	}
	fmt.Fprint(w, "\n")
	return w.Flush()
}
