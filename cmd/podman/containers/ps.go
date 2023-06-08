package containers

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tm "github.com/buger/goterm"
	"github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/utils"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	psDescription = "Prints out information about the containers"
	psCommand     = &cobra.Command{
		Use:               "ps [options]",
		Short:             "List containers",
		Long:              psDescription,
		RunE:              ps,
		Args:              validate.NoArgs,
		ValidArgsFunction: completion.AutocompleteNone,
		Example: `podman ps -a
  podman ps -a --format "{{.ID}}  {{.Image}}  {{.Labels}}  {{.Mounts}}"
  podman ps --size --sort names`,
	}

	psContainerCommand = &cobra.Command{
		Use:               psCommand.Use,
		Short:             psCommand.Short,
		Long:              psCommand.Long,
		RunE:              psCommand.RunE,
		Args:              psCommand.Args,
		ValidArgsFunction: psCommand.ValidArgsFunction,
		Example:           strings.ReplaceAll(psCommand.Example, "podman ps", "podman container ps"),
	}
)
var (
	listOpts = entities.ContainerListOptions{
		Filters: make(map[string][]string),
	}
	filters []string
	noTrunc bool
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: psCommand,
	})
	listFlagSet(psCommand)
	validate.AddLatestFlag(psCommand, &listOpts.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: psContainerCommand,
		Parent:  containerCmd,
	})
	listFlagSet(psContainerCommand)
	validate.AddLatestFlag(psContainerCommand, &listOpts.Latest)
}

func listFlagSet(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVarP(&listOpts.All, "all", "a", false, "Show all the containers, default is only running containers")
	flags.BoolVar(&listOpts.External, "external", false, "Show containers in storage not controlled by Podman")

	filterFlagName := "filter"
	flags.StringArrayVarP(&filters, filterFlagName, "f", []string{}, "Filter output based on conditions given")
	_ = cmd.RegisterFlagCompletionFunc(filterFlagName, common.AutocompletePsFilters)

	formatFlagName := "format"
	flags.StringVar(&listOpts.Format, formatFlagName, "", "Pretty-print containers to JSON or using a Go template")
	_ = cmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&psReporter{}))

	lastFlagName := "last"
	flags.IntVarP(&listOpts.Last, lastFlagName, "n", -1, "Print the n last created containers (all states)")
	_ = cmd.RegisterFlagCompletionFunc(lastFlagName, completion.AutocompleteNone)

	flags.BoolVar(&listOpts.Namespace, "ns", false, "Display namespace information")
	flags.BoolVar(&noTrunc, "no-trunc", false, "Display the extended information")
	flags.BoolVarP(&listOpts.Pod, "pod", "p", false, "Print the ID and name of the pod the containers are associated with")
	flags.BoolVarP(&listOpts.Quiet, "quiet", "q", false, "Print the numeric IDs of the containers only")
	flags.Bool("noheading", false, "Do not print headers")
	flags.BoolVarP(&listOpts.Size, "size", "s", false, "Display the total file sizes")
	flags.BoolVar(&listOpts.Sync, "sync", false, "Sync container state with OCI runtime")

	watchFlagName := "watch"
	flags.UintVarP(&listOpts.Watch, watchFlagName, "w", 0, "Watch the ps output on an interval in seconds")
	_ = cmd.RegisterFlagCompletionFunc(watchFlagName, completion.AutocompleteNone)

	sort := validate.Value(&listOpts.Sort, "command", "created", "id", "image", "names", "runningfor", "size", "status")
	sortFlagName := "sort"
	flags.Var(sort, sortFlagName, "Sort output by: "+sort.Choices())
	_ = cmd.RegisterFlagCompletionFunc(sortFlagName, common.AutocompletePsSort)

	flags.SetNormalizeFunc(utils.AliasFlags)
}
func checkFlags(c *cobra.Command) error {
	// latest, and last are mutually exclusive.
	if listOpts.Last >= 0 && listOpts.Latest {
		return errors.New("last and latest are mutually exclusive")
	}
	// Quiet conflicts with size and namespace and is overridden by a Go
	// template.
	if listOpts.Quiet {
		if listOpts.Size || listOpts.Namespace {
			return errors.New("quiet conflicts with size and namespace")
		}
	}
	// Size and namespace conflict with each other
	if listOpts.Size && listOpts.Namespace {
		return errors.New("size and namespace options conflict")
	}

	if listOpts.Watch > 0 && listOpts.Latest {
		return errors.New("the watch and latest flags cannot be used together")
	}
	podmanConfig := registry.PodmanConfig()
	if podmanConfig.ContainersConf.Engine.Namespace != "" {
		if c.Flag("storage").Changed && listOpts.External {
			return errors.New("--namespace and --external flags can not both be set")
		}
		listOpts.External = false
	}

	return nil
}

func jsonOut(responses []entities.ListContainer) error {
	type jsonFormat struct {
		entities.ListContainer
		Created int64
	}
	r := make([]jsonFormat, 0)
	for _, con := range responses {
		con.CreatedAt = units.HumanDuration(time.Since(con.Created)) + " ago"
		con.Status = psReporter{con}.Status()
		jf := jsonFormat{
			ListContainer: con,
			Created:       con.Created.Unix(),
		}
		r = append(r, jf)
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func quietOut(responses []entities.ListContainer) error {
	for _, r := range responses {
		id := r.ID
		if !noTrunc {
			id = id[0:12]
		}
		fmt.Println(id)
	}
	return nil
}

func getResponses() ([]entities.ListContainer, error) {
	responses, err := registry.ContainerEngine().ContainerList(registry.GetContext(), listOpts)
	if err != nil {
		return nil, err
	}
	if len(listOpts.Sort) > 0 {
		responses, err = entities.SortPsOutput(listOpts.Sort, responses)
		if err != nil {
			return nil, err
		}
	}
	return responses, nil
}

func ps(cmd *cobra.Command, _ []string) error {
	if err := checkFlags(cmd); err != nil {
		return err
	}
	for _, f := range filters {
		split := strings.SplitN(f, "=", 2)
		if len(split) == 1 {
			return fmt.Errorf("invalid filter %q", f)
		}
		listOpts.Filters[split[0]] = append(listOpts.Filters[split[0]], split[1])
	}
	listContainers, err := getResponses()
	if err != nil {
		return err
	}
	if len(listOpts.Sort) > 0 {
		listContainers, err = entities.SortPsOutput(listOpts.Sort, listContainers)
		if err != nil {
			return err
		}
	}

	switch {
	case report.IsJSON(listOpts.Format):
		return jsonOut(listContainers)
	case listOpts.Quiet && !cmd.Flags().Changed("format"):
		return quietOut(listContainers)
	}

	responses := make([]psReporter, 0, len(listContainers))
	for _, r := range listContainers {
		responses = append(responses, psReporter{r})
	}

	hdrs, format := createPsOut()

	var origin report.Origin
	noHeading, _ := cmd.Flags().GetBool("noheading")
	if cmd.Flags().Changed("format") {
		noHeading = noHeading || !report.HasTable(listOpts.Format)
		format = listOpts.Format
		origin = report.OriginUser
	} else {
		origin = report.OriginPodman
	}
	ns := strings.NewReplacer(".Namespaces.", ".")
	format = ns.Replace(format)

	rpt, err := report.New(os.Stdout, cmd.Name()).Parse(origin, format)
	if err != nil {
		return err
	}
	defer rpt.Flush()

	headers := func() error { return nil }
	if !noHeading {
		headers = func() error {
			return rpt.Execute(hdrs)
		}
	}

	switch {
	// Output table Watch > 0 will refresh screen
	case listOpts.Watch > 0:
		// responses will grow to the largest number of processes reported on, but will not thrash the gc
		var responses []psReporter
		for ; ; responses = responses[:0] {
			ctnrs, err := getResponses()
			if err != nil {
				return err
			}
			for _, r := range ctnrs {
				responses = append(responses, psReporter{r})
			}

			tm.Clear()
			tm.MoveCursor(1, 1)
			tm.Flush()

			if err := headers(); err != nil {
				return err
			}
			if err := rpt.Execute(responses); err != nil {
				return err
			}
			if err := rpt.Flush(); err != nil {
				// we usually do not care about Flush() failures but here do not loop if Flush() has failed
				return err
			}

			time.Sleep(time.Duration(listOpts.Watch) * time.Second)
		}
	default:
		if err := headers(); err != nil {
			return err
		}
		if err := rpt.Execute(responses); err != nil {
			return err
		}
	}
	return nil
}

// cannot use report.Headers() as it doesn't support structures as fields
func createPsOut() ([]map[string]string, string) {
	hdrs := report.Headers(psReporter{}, map[string]string{
		"Cgroup":       "cgroupns",
		"CreatedHuman": "created",
		"ID":           "container id",
		"IPC":          "ipc",
		"MNT":          "mnt",
		"NET":          "net",
		"Networks":     "networks",
		"PIDNS":        "pidns",
		"Pod":          "pod id",
		"PodName":      "podname", // undo camelcase space break
		"Restarts":     "restarts",
		"RunningFor":   "running for",
		"UTS":          "uts",
		"User":         "userns",
	})

	var row string
	if listOpts.Namespace {
		row = "{{.ID}}\t{{.Names}}\t{{.Pid}}\t{{.Namespaces.Cgroup}}\t{{.Namespaces.IPC}}\t{{.Namespaces.MNT}}\t{{.Namespaces.NET}}\t{{.Namespaces.PIDNS}}\t{{.Namespaces.User}}\t{{.Namespaces.UTS}}"
	} else {
		row = "{{.ID}}\t{{.Image}}\t{{.Command}}\t{{.CreatedHuman}}\t{{.Status}}\t{{.Ports}}\t{{.Names}}"

		if listOpts.Pod {
			row += "\t{{.Pod}}\t{{.PodName}}"
		}

		if listOpts.Size {
			row += "\t{{.Size}}"
		}
	}
	return hdrs, "{{range .}}" + row + "\n{{end -}}"
}

type psReporter struct {
	entities.ListContainer
}

// ImageID returns the ID of the container
func (l psReporter) ImageID() string {
	if !noTrunc {
		return l.ListContainer.ImageID[0:12]
	}
	return l.ListContainer.ImageID
}

// ID returns the ID of the container
func (l psReporter) ID() string {
	if !noTrunc {
		return l.ListContainer.ID[0:12]
	}
	return l.ListContainer.ID
}

// Pod returns the ID of the pod the container
// belongs to and appropriately truncates the ID
func (l psReporter) Pod() string {
	if !noTrunc && len(l.ListContainer.Pod) > 0 {
		return l.ListContainer.Pod[0:12]
	}
	return l.ListContainer.Pod
}

// Status returns the container status in the default ps output format.
func (l psReporter) Status() string {
	var state string
	switch l.ListContainer.State {
	case "running":
		t := units.HumanDuration(time.Since(time.Unix(l.StartedAt, 0)))
		state = "Up " + t
	case "exited", "stopped":
		t := units.HumanDuration(time.Since(time.Unix(l.ExitedAt, 0)))
		state = fmt.Sprintf("Exited (%d) %s ago", l.ExitCode, t)
	default:
		// Need to capitalize the first letter to match Docker.

		// strings.Title is deprecated since go 1.18
		// However for our use case it is still fine. The recommended replacement
		// is adding about 400kb binary size so let's keep using this for now.
		//nolint:staticcheck
		state = strings.Title(l.ListContainer.State)
	}
	hc := l.ListContainer.Status
	if hc != "" {
		state += " (" + hc + ")"
	}
	return state
}

func (l psReporter) Restarts() string {
	return strconv.Itoa(int(l.ListContainer.Restarts))
}

func (l psReporter) RunningFor() string {
	return l.CreatedHuman()
}

// Command returns the container command in string format
func (l psReporter) Command() string {
	command := strings.Join(l.ListContainer.Command, " ")
	if !noTrunc {
		if len(command) > 17 {
			return command[0:17] + "..."
		}
	}
	return command
}

// Size returns the rootfs and virtual sizes in human duration in
// and output form (string) suitable for ps
func (l psReporter) Size() string {
	if l.ListContainer.Size == nil {
		logrus.Errorf("Size format requires --size option")
		return ""
	}

	virt := units.HumanSizeWithPrecision(float64(l.ListContainer.Size.RootFsSize), 3)
	s := units.HumanSizeWithPrecision(float64(l.ListContainer.Size.RwSize), 3)
	return fmt.Sprintf("%s (virtual %s)", s, virt)
}

// Names returns the container name in string format
func (l psReporter) Names() string {
	return l.ListContainer.Names[0]
}

// Networks returns the container network names in string format
func (l psReporter) Networks() string {
	return strings.Join(l.ListContainer.Networks, ",")
}

// Ports converts from Portmappings to the string form
// required by ps
func (l psReporter) Ports() string {
	if len(l.ListContainer.Ports) < 1 {
		return ""
	}
	return portsToString(l.ListContainer.Ports)
}

// CreatedAt returns the container creation time in string format.  podman
// and docker both return a timestamped value for createdat
func (l psReporter) CreatedAt() string {
	return l.Created.String()
}

// CreateHuman allows us to output the created time in human readable format
func (l psReporter) CreatedHuman() string {
	return units.HumanDuration(time.Since(l.Created)) + " ago"
}

// Cgroup exposes .Namespaces.Cgroup
func (l psReporter) Cgroup() string {
	return l.Namespaces.Cgroup
}

// IPC exposes .Namespaces.IPC
func (l psReporter) IPC() string {
	return l.Namespaces.IPC
}

// MNT exposes .Namespaces.MNT
func (l psReporter) MNT() string {
	return l.Namespaces.MNT
}

// NET exposes .Namespaces.NET
func (l psReporter) NET() string {
	return l.Namespaces.NET
}

// PIDNS exposes .Namespaces.PIDNS
func (l psReporter) PIDNS() string {
	return l.Namespaces.PIDNS
}

// User exposes .Namespaces.User
func (l psReporter) User() string {
	return l.Namespaces.User
}

// UTS exposes .Namespaces.UTS
func (l psReporter) UTS() string {
	return l.Namespaces.UTS
}

// portsToString converts the ports used to a string of the from "port1, port2"
// and also groups a continuous list of ports into a readable format.
// The format is IP:HostPort(-Range)->ContainerPort(-Range)/Proto
func portsToString(ports []types.PortMapping) string {
	if len(ports) == 0 {
		return ""
	}
	sb := &strings.Builder{}
	for _, port := range ports {
		hostIP := port.HostIP
		if hostIP == "" {
			hostIP = "0.0.0.0"
		}
		protocols := strings.Split(port.Protocol, ",")
		for _, protocol := range protocols {
			if port.Range > 1 {
				fmt.Fprintf(sb, "%s:%d-%d->%d-%d/%s, ",
					hostIP, port.HostPort, port.HostPort+port.Range-1,
					port.ContainerPort, port.ContainerPort+port.Range-1, protocol)
			} else {
				fmt.Fprintf(sb, "%s:%d->%d/%s, ",
					hostIP, port.HostPort,
					port.ContainerPort, protocol)
			}
		}
	}
	display := sb.String()
	// make sure to trim the last ", " of the string
	return display[:len(display)-2]
}
