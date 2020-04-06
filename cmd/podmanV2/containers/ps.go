package containers

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"text/template"
	"time"

	tm "github.com/buger/goterm"
	"github.com/containers/buildah/pkg/formats"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/cmd/podmanV2/report"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	psDescription = "Prints out information about the containers"
	psCommand     = &cobra.Command{
		Use:     "ps",
		Args:    checkFlags,
		Short:   "List containers",
		Long:    psDescription,
		RunE:    ps,
		PreRunE: preRunE,
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
	defaultHeaders string = "CONTAINER ID\tIMAGE\tCOMMAND\tCREATED\tSTATUS\tPORTS\tNAMES"

//	CONTAINER ID  IMAGE                            COMMAND  CREATED     STATUS           PORTS  NAMES

)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: psCommand,
	})
	flags := psCommand.Flags()
	flags.BoolVarP(&listOpts.All, "all", "a", false, "Show all the containers, default is only running containers")
	flags.StringSliceVarP(&filters, "filter", "f", []string{}, "Filter output based on conditions given")
	flags.StringVar(&listOpts.Format, "format", "", "Pretty-print containers to JSON or using a Go template")
	flags.IntVarP(&listOpts.Last, "last", "n", -1, "Print the n last created containers (all states)")
	flags.BoolVarP(&listOpts.Latest, "latest", "l", false, "Show the latest container created (all states)")
	flags.BoolVar(&listOpts.Namespace, "namespace", false, "Display namespace information")
	flags.BoolVar(&listOpts.Namespace, "ns", false, "Display namespace information")
	flags.BoolVar(&noTrunc, "no-trunc", false, "Display the extended information")
	flags.BoolVarP(&listOpts.Pod, "pod", "p", false, "Print the ID and name of the pod the containers are associated with")
	flags.BoolVarP(&listOpts.Quiet, "quiet", "q", false, "Print the numeric IDs of the containers only")
	flags.BoolVarP(&listOpts.Size, "size", "s", false, "Display the total file sizes")
	flags.StringVar(&listOpts.Sort, "sort", "created", "Sort output by command, created, id, image, names, runningfor, size, or status")
	flags.BoolVar(&listOpts.Sync, "sync", false, "Sync container state with OCI runtime")
	flags.UintVarP(&listOpts.Watch, "watch", "w", 0, "Watch the ps output on an interval in seconds")
	if registry.IsRemote() {
		_ = flags.MarkHidden("latest")
	}
}
func checkFlags(c *cobra.Command, args []string) error {
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
		if c.Flag("format").Changed && listOpts.Format != formats.JSONString {
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
	return nil
}

func jsonOut(responses []entities.ListContainer) error {
	b, err := json.MarshalIndent(responses, "", "  ")
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
	// []string to map[string][]string
	for _, f := range filters {
		split := strings.SplitN(f, "=", 2)
		if len(split) == 1 {
			return errors.Errorf("invalid filter %q", f)
		}
		listOpts.Filters[split[0]] = append(listOpts.Filters[split[0]], split[1])
	}
	responses, err := getResponses()
	if err != nil {
		return err
	}
	if len(listOpts.Sort) > 0 {
		responses, err = entities.SortPsOutput(listOpts.Sort, responses)
		if err != nil {
			return err
		}
	}
	if listOpts.Format == "json" {
		return jsonOut(responses)
	}
	if listOpts.Quiet {
		return quietOut(responses)
	}
	headers, row := createPsOut()
	if cmd.Flag("format").Changed {
		row = listOpts.Format
		if !strings.HasPrefix(row, "\n") {
			row += "\n"
		}
	}
	format := "{{range . }}" + row + "{{end}}"
	if !listOpts.Quiet && !cmd.Flag("format").Changed {
		format = headers + format
	}
	funcs := report.AppendFuncMap(psFuncMap)
	tmpl, err := template.New("listPods").Funcs(funcs).Parse(format)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 8, 2, 2, ' ', 0)
	if listOpts.Watch > 0 {
		for {
			tm.Clear()
			tm.MoveCursor(1, 1)
			tm.Flush()
			responses, err := getResponses()
			if err != nil {
				return err
			}
			if err := tmpl.Execute(w, responses); err != nil {
				return nil
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
		return w.Flush()
	}
	return nil
}

func createPsOut() (string, string) {
	var row string
	if listOpts.Namespace {
		headers := "CONTAINER ID\tNAMES\tPID\tCGROUPNS\tIPC\tMNT\tNET\tPIDN\tUSERNS\tUTS\n"
		row := "{{.ID}}\t{{names .Names}}\t{{.Pid}}\t{{.Namespaces.Cgroup}}\t{{.Namespaces.IPC}}\t{{.Namespaces.MNT}}\t{{.Namespaces.NET}}\t{{.Namespaces.PIDNS}}\t{{.Namespaces.User}}\t{{.Namespaces.UTS}}\n"
		return headers, row
	}
	headers := defaultHeaders
	if noTrunc {
		row += "{{.ID}}"
	} else {
		row += "{{slice .ID 0 12}}"
	}
	row += "\t{{.Image}}\t{{cmd .Command}}\t{{humanDuration .Created}}\t{{state .}}\t{{ports .Ports}}\t{{names .Names}}"

	if listOpts.Pod {
		headers += "\tPOD ID\tPODNAME"
		if noTrunc {
			row += "\t{{.Pod}}"
		} else {
			row += "\t{{slice .Pod 0 12}}"
		}
		row += "\t{{.PodName}}"
	}

	if listOpts.Size {
		headers += "\tSIZE"
		row += "\t{{consize .Size}}"
	}
	if !strings.HasSuffix(headers, "\n") {
		headers += "\n"
	}
	if !strings.HasSuffix(row, "\n") {
		row += "\n"
	}
	return headers, row
}

var psFuncMap = template.FuncMap{
	"cmd": func(conCommand []string) string {
		return strings.Join(conCommand, " ")
	},
	"state": func(con entities.ListContainer) string {
		var state string
		switch con.State {
		case "running":
			t := units.HumanDuration(time.Since(time.Unix(con.StartedAt, 0)))
			state = "Up " + t + " ago"
		case "configured":
			state = "Created"
		case "exited":
			t := units.HumanDuration(time.Since(time.Unix(con.ExitedAt, 0)))
			state = fmt.Sprintf("Exited (%d) %s ago", con.ExitCode, t)
		default:
			state = con.State
		}
		return state
	},
	"ports": func(ports []ocicni.PortMapping) string {
		if len(ports) == 0 {
			return ""
		}
		return portsToString(ports)
	},
	"names": func(names []string) string {
		return names[0]
	},
	"consize": func(csize shared.ContainerSize) string {
		virt := units.HumanSizeWithPrecision(float64(csize.RootFsSize), 3)
		s := units.HumanSizeWithPrecision(float64(csize.RwSize), 3)
		return fmt.Sprintf("%s (virtual %s)", s, virt)
	},
}

// portsToString converts the ports used to a string of the from "port1, port2"
// and also groups a continuous list of ports into a readable format.
func portsToString(ports []ocicni.PortMapping) string {
	type portGroup struct {
		first int32
		last  int32
	}
	var portDisplay []string
	if len(ports) == 0 {
		return ""
	}
	//Sort the ports, so grouping continuous ports become easy.
	sort.Slice(ports, func(i, j int) bool {
		return comparePorts(ports[i], ports[j])
	})

	// portGroupMap is used for grouping continuous ports.
	portGroupMap := make(map[string]*portGroup)
	var groupKeyList []string

	for _, v := range ports {

		hostIP := v.HostIP
		if hostIP == "" {
			hostIP = "0.0.0.0"
		}
		// If hostPort and containerPort are not same, consider as individual port.
		if v.ContainerPort != v.HostPort {
			portDisplay = append(portDisplay, fmt.Sprintf("%s:%d->%d/%s", hostIP, v.HostPort, v.ContainerPort, v.Protocol))
			continue
		}

		portMapKey := fmt.Sprintf("%s/%s", hostIP, v.Protocol)

		portgroup, ok := portGroupMap[portMapKey]
		if !ok {
			portGroupMap[portMapKey] = &portGroup{first: v.ContainerPort, last: v.ContainerPort}
			// This list is required to traverse portGroupMap.
			groupKeyList = append(groupKeyList, portMapKey)
			continue
		}

		if portgroup.last == (v.ContainerPort - 1) {
			portgroup.last = v.ContainerPort
			continue
		}
	}
	// For each portMapKey, format group list and appned to output string.
	for _, portKey := range groupKeyList {
		group := portGroupMap[portKey]
		portDisplay = append(portDisplay, formatGroup(portKey, group.first, group.last))
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

// formatGroup returns the group as <IP:startPort:lastPort->startPort:lastPort/Proto>
// e.g 0.0.0.0:1000-1006->1000-1006/tcp.
func formatGroup(key string, start, last int32) string {
	parts := strings.Split(key, "/")
	groupType := parts[0]
	var ip string
	if len(parts) > 1 {
		ip = parts[0]
		groupType = parts[1]
	}
	group := strconv.Itoa(int(start))
	if start != last {
		group = fmt.Sprintf("%s-%d", group, last)
	}
	if ip != "" {
		group = fmt.Sprintf("%s:%s->%s", ip, group, group)
	}
	return fmt.Sprintf("%s/%s", group, groupType)
}
