package main

import (
	"fmt"
	"html/template"
	"os"
	"reflect"
	"strings"
	"text/tabwriter"
	"time"

	tm "github.com/buger/goterm"
	"github.com/containers/buildah/pkg/formats"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	podStatsCommand     cliconfig.PodStatsValues
	podStatsDescription = `For each specified pod this command will display percentage of CPU, memory, network I/O, block I/O and PIDs for containers in one the pods.`

	_podStatsCommand = &cobra.Command{
		Use:   "stats [flags] [POD...]",
		Short: "Display a live stream of resource usage statistics for the containers in one or more pods",
		Long:  podStatsDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			podStatsCommand.InputArgs = args
			podStatsCommand.GlobalFlags = MainGlobalOpts
			podStatsCommand.Remote = remoteclient
			return podStatsCmd(&podStatsCommand)
		},
		Example: `podman stats -a --no-stream
  podman stats --no-reset ctrID
  podman stats --no-stream --format "table {{.ID}} {{.Name}} {{.MemUsage}}" ctrID`,
	}
)

func init() {
	podStatsCommand.Command = _podStatsCommand
	podStatsCommand.SetHelpTemplate(HelpTemplate())
	podStatsCommand.SetUsageTemplate(UsageTemplate())
	flags := podStatsCommand.Flags()
	flags.BoolVarP(&podStatsCommand.All, "all", "a", false, "Provide stats for all running pods")
	flags.StringVar(&podStatsCommand.Format, "format", "", "Pretty-print container statistics to JSON or using a Go template")
	flags.BoolVarP(&podStatsCommand.Latest, "latest", "l", false, "Provide stats on the latest pod podman is aware of")
	flags.BoolVar(&podStatsCommand.NoStream, "no-stream", false, "Disable streaming stats and only pull the first result, default setting is false")
	flags.BoolVar(&podStatsCommand.NoReset, "no-reset", false, "Disable resetting the screen between intervals")
	markFlagHiddenForRemoteClient("latest", flags)
}

func podStatsCmd(c *cliconfig.PodStatsValues) error {

	if os.Geteuid() != 0 {
		return errors.New("stats is not supported in rootless mode")
	}

	format := c.Format
	all := c.All
	latest := c.Latest
	ctr := 0
	if all {
		ctr += 1
	}
	if latest {
		ctr += 1
	}
	if len(c.InputArgs) > 0 {
		ctr += 1
	}

	if ctr > 1 {
		return errors.Errorf("--all, --latest and containers cannot be used together")
	} else if ctr == 0 {
		// If user didn't specify, imply --all
		all = true
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	times := -1
	if c.NoStream {
		times = 1
	}

	pods, err := runtime.GetStatPods(c)
	if err != nil {
		return errors.Wrapf(err, "unable to get a list of pods")
	}
	// First we need to get an initial pass of pod/ctr stats (these are not printed)
	var podStats []*adapter.PodContainerStats
	for _, p := range pods {
		cons, err := p.AllContainersByID()
		if err != nil {
			return err
		}
		emptyStats := make(map[string]*libpod.ContainerStats)
		// Iterate the pods container ids and make blank stats for them
		for _, c := range cons {
			emptyStats[c] = &libpod.ContainerStats{}
		}
		ps := adapter.PodContainerStats{
			Pod:            p,
			ContainerStats: emptyStats,
		}
		podStats = append(podStats, &ps)
	}

	// Create empty container stat results for our first pass
	var previousPodStats []*adapter.PodContainerStats
	for _, p := range pods {
		cs := make(map[string]*libpod.ContainerStats)
		pcs := adapter.PodContainerStats{
			Pod:            p,
			ContainerStats: cs,
		}
		previousPodStats = append(previousPodStats, &pcs)
	}

	step := 1
	if times == -1 {
		times = 1
		step = 0
	}

	headerNames := make(map[string]string)
	if c.Format != "" {
		// Make a map of the field names for the headers
		v := reflect.ValueOf(podStatOut{})
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			value := strings.ToUpper(splitCamelCase(t.Field(i).Name))
			switch value {
			case "CPU":
				value = value + " %"
			case "MEM":
				value = value + " %"
			case "MEM USAGE":
				value = "MEM USAGE / LIMIT"
			}
			headerNames[t.Field(i).Name] = value
		}
	}

	for i := 0; i < times; i += step {
		var newStats []*adapter.PodContainerStats
		for _, p := range pods {
			prevStat := getPreviousPodContainerStats(p.ID(), previousPodStats)
			newPodStats, err := p.GetPodStats(prevStat)
			if errors.Cause(err) == define.ErrNoSuchPod {
				continue
			}
			if err != nil {
				return err
			}
			newPod := adapter.PodContainerStats{
				Pod:            p,
				ContainerStats: newPodStats,
			}
			newStats = append(newStats, &newPod)
		}
		//Output
		if strings.ToLower(format) != formats.JSONString && !c.NoReset {
			tm.Clear()
			tm.MoveCursor(1, 1)
			tm.Flush()
		}
		if strings.ToLower(format) == formats.JSONString {
			outputJson(newStats)

		} else {
			results := podContainerStatsToPodStatOut(newStats)
			if len(format) == 0 {
				outputToStdOut(results)
			} else {
				if err := printPSFormat(c.Format, results, headerNames); err != nil {
					return err
				}
			}
		}
		time.Sleep(time.Second)
		previousPodStats := new([]*libpod.PodContainerStats)
		if err := libpod.JSONDeepCopy(newStats, previousPodStats); err != nil {
			return err
		}
		pods, err = runtime.GetStatPods(c)
		if err != nil {
			return err
		}
	}

	return nil
}

func podContainerStatsToPodStatOut(stats []*adapter.PodContainerStats) []*podStatOut {
	var out []*podStatOut
	for _, p := range stats {
		for _, c := range p.ContainerStats {
			o := podStatOut{
				CPU:      floatToPercentString(c.CPU),
				MemUsage: combineHumanValues(c.MemUsage, c.MemLimit),
				Mem:      floatToPercentString(c.MemPerc),
				NetIO:    combineHumanValues(c.NetInput, c.NetOutput),
				BlockIO:  combineHumanValues(c.BlockInput, c.BlockOutput),
				PIDS:     pidsToString(c.PIDs),
				CID:      c.ContainerID[:12],
				Name:     c.Name,
				Pod:      p.Pod.ID()[:12],
			}
			out = append(out, &o)
		}
	}
	return out
}

type podStatOut struct {
	CPU      string
	MemUsage string
	Mem      string
	NetIO    string
	BlockIO  string
	PIDS     string
	Pod      string
	CID      string
	Name     string
}

func printPSFormat(format string, stats []*podStatOut, headerNames map[string]string) error {
	if len(stats) == 0 {
		return nil
	}

	// Use a tabwriter to align column format
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
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
	for _, container := range stats {
		if err := dataTmpl.Execute(w, container); err != nil {
			return err
		}
		fmt.Fprintln(w, "")
	}
	// Flush the writer
	return w.Flush()

}

func outputToStdOut(stats []*podStatOut) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	outFormat := "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n"
	fmt.Fprintf(w, outFormat, "POD", "CID", "NAME", "CPU %", "MEM USAGE/ LIMIT", "MEM %", "NET IO", "BLOCK IO", "PIDS")
	for _, i := range stats {
		if len(stats) == 0 {
			fmt.Fprintf(w, outFormat, i.Pod, "--", "--", "--", "--", "--", "--", "--", "--")
		} else {
			fmt.Fprintf(w, outFormat, i.Pod, i.CID, i.Name, i.CPU, i.MemUsage, i.Mem, i.NetIO, i.BlockIO, i.PIDS)
		}
	}
	w.Flush()
}

func getPreviousPodContainerStats(podID string, prev []*adapter.PodContainerStats) map[string]*libpod.ContainerStats {
	for _, p := range prev {
		if podID == p.Pod.ID() {
			return p.ContainerStats
		}
	}
	return map[string]*libpod.ContainerStats{}
}

func outputJson(stats []*adapter.PodContainerStats) error {
	b, err := json.MarshalIndent(&stats, "", "     ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func getPodsByList(podList []string, r *libpod.Runtime) ([]*libpod.Pod, error) {
	var (
		pods []*libpod.Pod
	)
	for _, p := range podList {
		pod, err := r.LookupPod(p)
		if err != nil {
			return nil, err
		}
		pods = append(pods, pod)
	}
	return pods, nil
}
