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
	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const (
	hid      = "CONTAINER ID"
	himage   = "IMAGE"
	hcommand = "COMMAND"
	hcreated = "CREATED"
	hstatus  = "STATUS"
	hports   = "PORTS"
	hnames   = "NAMES"
	hsize    = "SIZE"
	hinfra   = "IS INFRA" //nolint
	hpod     = "POD"
	hpodname = "POD NAME"
	nspid    = "PID"
	nscgroup = "CGROUPNS"
	nsipc    = "IPC"
	nsmnt    = "MNT"
	nsnet    = "NET"
	nspidns  = "PIDNS"
	nsuserns = "USERNS"
	nsuts    = "UTS"
)

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
	var (
		watch   bool
		runtime *adapter.LocalRuntime
		err     error
	)

	if c.Watch > 0 {
		watch = true
	}

	if c.Watch > 0 && c.Latest {
		return errors.New("the watch and latest flags cannot be used together")
	}

	if err := checkFlagsPassed(c); err != nil {
		return errors.Wrapf(err, "error with flags passed")
	}
	if !c.Size {
		runtime, err = adapter.GetRuntimeNoStore(getContext(), &c.PodmanCommand)
	} else {
		runtime, err = adapter.GetRuntime(getContext(), &c.PodmanCommand)
	}
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}

	defer runtime.DeferredShutdown(false)

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
	// Filter forces all
	if len(c.Filter) > 0 {
		c.All = true
	}
	// Quiet conflicts with size and namespace and is overridden by a Go
	// template.
	if c.Quiet {
		if c.Size || c.Namespace {
			return errors.Errorf("quiet conflicts with size and namespace")
		}
		if c.Flag("format").Changed && c.Format != formats.JSONString {
			// Quiet is overridden by Go template output.
			c.Quiet = false
		}
	}
	// Size and namespace conflict with each other
	if c.Size && c.Namespace {
		return errors.Errorf("size and namespace options conflict")
	}
	return nil
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
	if err != nil {
		return err
	}
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
			fmt.Fprintf(w, "\t%s\t%s", hpod, hpodname)
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
				fmt.Fprintf(w, "\t%s\t%s", container.Pod, container.PodName)
			}
			//User wants size info
			if opts.Size {
				var size string
				if container.Size == nil {
					size = units.HumanSizeWithPrecision(0, 0)
				} else {
					size = units.HumanSizeWithPrecision(float64(container.Size.RwSize), 3) + " (virtual " + units.HumanSizeWithPrecision(float64(container.Size.RootFsSize), 3) + ")"
				}
				fmt.Fprintf(w, "\t%s", size)
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
