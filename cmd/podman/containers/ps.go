package containers

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"text/template"
	"time"

	tm "github.com/buger/goterm"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/utils"
	"github.com/containers/podman/v2/cmd/podman/validate"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	psDescription = "Prints out information about the containers"
	psCommand     = &cobra.Command{
		Use:   "ps [options]",
		Args:  validate.NoArgs,
		Short: "List containers",
		Long:  psDescription,
		RunE:  ps,
		Example: `podman ps -a
  podman ps -a --format "{{.ID}}  {{.Image}}  {{.Labels}}  {{.Mounts}}"
  podman ps --size --sort names`,
	}
)
var (
	listOpts = entities.ContainerListOptions{
		Filters: make(map[string][]string),
	}
	filters        []string
	noTrunc        bool
	defaultHeaders = "CONTAINER ID\tIMAGE\tCOMMAND\tCREATED\tSTATUS\tPORTS\tNAMES"
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: psCommand,
	})
	listFlagSet(psCommand.Flags())
	validate.AddLatestFlag(psCommand, &listOpts.Latest)
}

func listFlagSet(flags *pflag.FlagSet) {
	flags.BoolVarP(&listOpts.All, "all", "a", false, "Show all the containers, default is only running containers")
	flags.StringSliceVarP(&filters, "filter", "f", []string{}, "Filter output based on conditions given")
	flags.BoolVar(&listOpts.Storage, "external", false, "Show containers in storage not controlled by Podman")
	flags.StringVar(&listOpts.Format, "format", "", "Pretty-print containers to JSON or using a Go template")
	flags.IntVarP(&listOpts.Last, "last", "n", -1, "Print the n last created containers (all states)")
	flags.BoolVar(&listOpts.Namespace, "ns", false, "Display namespace information")
	flags.BoolVar(&noTrunc, "no-trunc", false, "Display the extended information")
	flags.BoolVarP(&listOpts.Pod, "pod", "p", false, "Print the ID and name of the pod the containers are associated with")
	flags.BoolVarP(&listOpts.Quiet, "quiet", "q", false, "Print the numeric IDs of the containers only")
	flags.BoolVarP(&listOpts.Size, "size", "s", false, "Display the total file sizes")
	flags.BoolVar(&listOpts.Sync, "sync", false, "Sync container state with OCI runtime")
	flags.UintVarP(&listOpts.Watch, "watch", "w", 0, "Watch the ps output on an interval in seconds")

	sort := validate.Value(&listOpts.Sort, "command", "created", "id", "image", "names", "runningfor", "size", "status")
	flags.Var(sort, "sort", "Sort output by: "+sort.Choices())
	flags.SetNormalizeFunc(utils.AliasFlags)
}
func checkFlags(c *cobra.Command) error {
	// latest, and last are mutually exclusive.
	if listOpts.Last >= 0 && listOpts.Latest {
		return errors.Errorf("last and latest are mutually exclusive")
	}
	// Filter on status forces all
	for _, filter := range filters {
		splitFilter := strings.SplitN(filter, "=", 2)
		if strings.ToLower(splitFilter[0]) == "status" {
			listOpts.All = true
			break
		}
	}
	// Quiet conflicts with size and namespace and is overridden by a Go
	// template.
	if listOpts.Quiet {
		if listOpts.Size || listOpts.Namespace {
			return errors.Errorf("quiet conflicts with size and namespace")
		}
		if c.Flag("format").Changed && !report.IsJSON(listOpts.Format) {
			// Quiet is overridden by Go template output.
			listOpts.Quiet = false
		}
	}
	// Size and namespace conflict with each other
	if listOpts.Size && listOpts.Namespace {
		return errors.Errorf("size and namespace options conflict")
	}

	if listOpts.Watch > 0 && listOpts.Latest {
		return errors.New("the watch and latest flags cannot be used together")
	}
	cfg := registry.PodmanConfig()
	if cfg.Engine.Namespace != "" {
		if c.Flag("storage").Changed && listOpts.Storage {
			return errors.New("--namespace and --storage flags can not both be set")
		}
		listOpts.Storage = false
	}

	return nil
}

func jsonOut(responses []entities.ListContainer) error {
	r := make([]entities.ListContainer, 0)
	for _, con := range responses {
		con.CreatedAt = units.HumanDuration(time.Since(time.Unix(con.Created, 0))) + " ago"
		con.Status = psReporter{con}.Status()
		r = append(r, con)
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

func ps(cmd *cobra.Command, args []string) error {
	if err := checkFlags(cmd); err != nil {
		return err
	}
	for _, f := range filters {
		split := strings.SplitN(f, "=", 2)
		if len(split) == 1 {
			return errors.Errorf("invalid filter %q", f)
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
	case listOpts.Quiet:
		return quietOut(listContainers)
	}
	// Output table Watch > 0 will refresh screen

	responses := make([]psReporter, 0, len(listContainers))
	for _, r := range listContainers {
		responses = append(responses, psReporter{r})
	}

	var headers, format string
	if cmd.Flags().Changed("format") {
		headers = ""
		format = report.NormalizeFormat(listOpts.Format)
	} else {
		headers, format = createPsOut()
	}
	format = headers + "{{range . }}" + format + "{{end}}"

	tmpl, err := template.New("listContainers").Parse(format)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 8, 2, 2, ' ', 0)
	defer w.Flush()

	if listOpts.Watch > 0 {
		for {
			var responses []psReporter
			tm.Clear()
			tm.MoveCursor(1, 1)
			tm.Flush()

			if ctnrs, err := getResponses(); err != nil {
				return err
			} else {
				for _, r := range ctnrs {
					responses = append(responses, psReporter{r})
				}
			}

			if err := tmpl.Execute(w, responses); err != nil {
				return err
			}
			if err := w.Flush(); err != nil {
				return err
			}
			time.Sleep(time.Duration(listOpts.Watch) * time.Second)
			tm.Clear()
			tm.MoveCursor(1, 1)
			tm.Flush()
		}
	} else if listOpts.Watch < 1 {
		if err := tmpl.Execute(w, responses); err != nil {
			return err
		}
	}
	return nil
}

// cannot use report.Headers() as it doesn't support structures as fields
func createPsOut() (string, string) {
	var row string
	if listOpts.Namespace {
		headers := "CONTAINER ID\tNAMES\tPID\tCGROUPNS\tIPC\tMNT\tNET\tPIDNS\tUSERNS\tUTS\n"
		row := "{{.ID}}\t{{.Names}}\t{{.Pid}}\t{{.Namespaces.Cgroup}}\t{{.Namespaces.IPC}}\t{{.Namespaces.MNT}}\t{{.Namespaces.NET}}\t{{.Namespaces.PIDNS}}\t{{.Namespaces.User}}\t{{.Namespaces.UTS}}\n"
		return headers, row
	}
	headers := defaultHeaders
	row += "{{.ID}}"
	row += "\t{{.Image}}\t{{.Command}}\t{{.CreatedHuman}}\t{{.Status}}\t{{.Ports}}\t{{.Names}}"

	if listOpts.Pod {
		headers += "\tPOD ID\tPODNAME"
		row += "\t{{.Pod}}\t{{.PodName}}"
	}

	if listOpts.Size {
		headers += "\tSIZE"
		row += "\t{{.Size}}"
	}

	headers = report.NormalizeFormat(headers)
	row = report.NormalizeFormat(row)
	return headers, row
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

// State returns the container state in human duration
func (l psReporter) State() string {
	var state string
	switch l.ListContainer.State {
	case "running":
		t := units.HumanDuration(time.Since(time.Unix(l.StartedAt, 0)))
		state = "Up " + t + " ago"
	case "configured":
		state = "Created"
	case "exited", "stopped":
		t := units.HumanDuration(time.Since(time.Unix(l.ExitedAt, 0)))
		state = fmt.Sprintf("Exited (%d) %s ago", l.ExitCode, t)
	default:
		state = l.ListContainer.State
	}
	return state
}

// Status is a synonym for State()
func (l psReporter) Status() string {
	return l.State()
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
	virt := units.HumanSizeWithPrecision(float64(l.ListContainer.Size.RootFsSize), 3)
	s := units.HumanSizeWithPrecision(float64(l.ListContainer.Size.RwSize), 3)
	return fmt.Sprintf("%s (virtual %s)", s, virt)
}

// Names returns the container name in string format
func (l psReporter) Names() string {
	return l.ListContainer.Names[0]
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
	return time.Unix(l.Created, 0).String()
}

// CreateHuman allows us to output the created time in human readable format
func (l psReporter) CreatedHuman() string {
	return units.HumanDuration(time.Since(time.Unix(l.Created, 0))) + " ago"
}

// portsToString converts the ports used to a string of the from "port1, port2"
// and also groups a continuous list of ports into a readable format.
func portsToString(ports []ocicni.PortMapping) string {
	if len(ports) == 0 {
		return ""
	}
	// Sort the ports, so grouping continuous ports become easy.
	sort.Slice(ports, func(i, j int) bool {
		return comparePorts(ports[i], ports[j])
	})

	portGroups := [][]ocicni.PortMapping{}
	currentGroup := []ocicni.PortMapping{}
	for i, v := range ports {
		var prevPort, nextPort *int32
		if i > 0 {
			prevPort = &ports[i-1].ContainerPort
		}
		if i+1 < len(ports) {
			nextPort = &ports[i+1].ContainerPort
		}

		port := v.ContainerPort

		// Helper functions
		addToCurrentGroup := func(x ocicni.PortMapping) {
			currentGroup = append(currentGroup, x)
		}

		addToPortGroup := func(x ocicni.PortMapping) {
			portGroups = append(portGroups, []ocicni.PortMapping{x})
		}

		finishCurrentGroup := func() {
			portGroups = append(portGroups, currentGroup)
			currentGroup = []ocicni.PortMapping{}
		}

		// Single entry slice
		if prevPort == nil && nextPort == nil {
			addToPortGroup(v)
		}

		// Start of the slice with len > 0
		if prevPort == nil && nextPort != nil {
			isGroup := *nextPort-1 == port

			if isGroup {
				// Start with a group
				addToCurrentGroup(v)
			} else {
				// Start with single item
				addToPortGroup(v)
			}

			continue
		}

		// Middle of the slice with len > 0
		if prevPort != nil && nextPort != nil {
			currentIsGroup := *prevPort+1 == port
			nextIsGroup := *nextPort-1 == port

			if currentIsGroup {
				// Maybe in the middle of a group
				addToCurrentGroup(v)

				if !nextIsGroup {
					// End of a group
					finishCurrentGroup()
				}
			} else if nextIsGroup {
				// Start of a new group
				addToCurrentGroup(v)
			} else {
				// No group at all
				addToPortGroup(v)
			}

			continue
		}

		// End of the slice with len > 0
		if prevPort != nil && nextPort == nil {
			isGroup := *prevPort+1 == port

			if isGroup {
				// End group
				addToCurrentGroup(v)
				finishCurrentGroup()
			} else {
				// End single item
				addToPortGroup(v)
			}
		}
	}

	portDisplay := []string{}
	for _, group := range portGroups {
		if len(group) == 0 {
			// Usually should not happen, but better do not crash.
			continue
		}

		first := group[0]

		hostIP := first.HostIP
		if hostIP == "" {
			hostIP = "0.0.0.0"
		}

		// Single mappings
		if len(group) == 1 {
			portDisplay = append(portDisplay,
				fmt.Sprintf(
					"%s:%d->%d/%s",
					hostIP, first.HostPort, first.ContainerPort, first.Protocol,
				),
			)
			continue
		}

		// Group mappings
		last := group[len(group)-1]
		portDisplay = append(portDisplay, formatGroup(
			fmt.Sprintf("%s/%s", hostIP, first.Protocol),
			first.HostPort, last.HostPort,
			first.ContainerPort, last.ContainerPort,
		))
	}
	return strings.Join(portDisplay, ", ")
}

func comparePorts(i, j ocicni.PortMapping) bool {
	if i.ContainerPort != j.ContainerPort {
		return i.ContainerPort < j.ContainerPort
	}

	if i.HostIP != j.HostIP {
		return i.HostIP < j.HostIP
	}

	if i.HostPort != j.HostPort {
		return i.HostPort < j.HostPort
	}

	return i.Protocol < j.Protocol
}

// formatGroup returns the group in the format:
// <IP:firstHost:lastHost->firstCtr:lastCtr/Proto>
// e.g 0.0.0.0:1000-1006->2000-2006/tcp.
func formatGroup(key string, firstHost, lastHost, firstCtr, lastCtr int32) string {
	parts := strings.Split(key, "/")
	groupType := parts[0]
	var ip string
	if len(parts) > 1 {
		ip = parts[0]
		groupType = parts[1]
	}

	group := func(first, last int32) string {
		group := strconv.Itoa(int(first))
		if first != last {
			group = fmt.Sprintf("%s-%d", group, last)
		}
		return group
	}
	hostGroup := group(firstHost, lastHost)
	ctrGroup := group(firstCtr, lastCtr)

	return fmt.Sprintf("%s:%s->%s/%s", ip, hostGroup, ctrGroup, groupType)
}
