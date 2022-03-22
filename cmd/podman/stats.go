package main

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	tm "github.com/buger/goterm"
	"github.com/containers/buildah/pkg/formats"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/cgroups"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type statsOutputParams struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	CPUPerc  string `json:"cpu_percent"`
	MemUsage string `json:"mem_usage"`
	MemPerc  string `json:"mem_percent"`
	NetIO    string `json:"netio"`
	BlockIO  string `json:"blocki"`
	PIDS     string `json:"pids"`
}

var (
	statsCommand cliconfig.StatsValues

	statsDescription = "Display percentage of CPU, memory, network I/O, block I/O and PIDs for one or more containers."
	_statsCommand    = &cobra.Command{
		Use:   "stats [flags] [CONTAINER...]",
		Short: "Display a live stream of container resource usage statistics",
		Long:  statsDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			statsCommand.InputArgs = args
			statsCommand.GlobalFlags = MainGlobalOpts
			statsCommand.Remote = remoteclient
			return statsCmd(&statsCommand)
		},
		Example: `podman stats --all --no-stream
  podman stats ctrID
  podman stats --no-stream --format "table {{.ID}} {{.Name}} {{.MemUsage}}" ctrID`,
	}
)

func init() {
	statsCommand.Command = _statsCommand
	statsCommand.SetHelpTemplate(HelpTemplate())
	statsCommand.SetUsageTemplate(UsageTemplate())
	flags := statsCommand.Flags()
	flags.BoolVarP(&statsCommand.All, "all", "a", false, "Show all containers. Only running containers are shown by default. The default is false")
	flags.StringVar(&statsCommand.Format, "format", "", "Pretty-print container statistics to JSON or using a Go template")
	flags.BoolVarP(&statsCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.BoolVar(&statsCommand.NoReset, "no-reset", false, "Disable resetting the screen between intervals")
	flags.BoolVar(&statsCommand.NoStream, "no-stream", false, "Disable streaming stats and only pull the first result, default setting is false")
	markFlagHiddenForRemoteClient("latest", flags)
}

func statsCmd(c *cliconfig.StatsValues) error {
	if rootless.IsRootless() {
		unified, err := cgroups.IsCgroup2UnifiedMode()
		if err != nil {
			return err
		}
		if !unified {
			return errors.New("stats is not supported in rootless mode without cgroups v2")
		}
	}

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
	}

	runtime, err := libpodruntime.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.DeferredShutdown(false)

	times := -1
	if c.NoStream {
		times = 1
	}

	var ctrs []*libpod.Container

	containerFunc := runtime.GetRunningContainers
	if len(c.InputArgs) > 0 {
		containerFunc = func() ([]*libpod.Container, error) { return runtime.GetContainersByList(c.InputArgs) }
	} else if latest {
		containerFunc = func() ([]*libpod.Container, error) {
			lastCtr, err := runtime.GetLatestContainer()
			if err != nil {
				return nil, err
			}
			return []*libpod.Container{lastCtr}, nil
		}
	} else if all {
		containerFunc = runtime.GetAllContainers
	}

	ctrs, err = containerFunc()
	if err != nil {
		return errors.Wrapf(err, "unable to get list of containers")
	}

	containerStats := map[string]*libpod.ContainerStats{}
	for _, ctr := range ctrs {
		initialStats, err := ctr.GetContainerStats(nil)
		if err != nil {
			// when doing "all", dont worry about containers that are not running
			cause := errors.Cause(err)
			if c.All && (cause == define.ErrCtrRemoved || cause == define.ErrNoSuchCtr || cause == define.ErrCtrStateInvalid) {
				continue
			}
			if cause == cgroups.ErrCgroupV1Rootless {
				err = cause
			}
			return err
		}
		containerStats[ctr.ID()] = initialStats
	}

	format := genStatsFormat(c.Format)

	step := 1
	if times == -1 {
		times = 1
		step = 0
	}
	for i := 0; i < times; i += step {
		reportStats := []*libpod.ContainerStats{}
		for _, ctr := range ctrs {
			id := ctr.ID()
			if _, ok := containerStats[ctr.ID()]; !ok {
				initialStats, err := ctr.GetContainerStats(nil)
				if errors.Cause(err) == define.ErrCtrRemoved || errors.Cause(err) == define.ErrNoSuchCtr || errors.Cause(err) == define.ErrCtrStateInvalid {
					// skip dealing with a container that is gone
					continue
				}
				if err != nil {
					return err
				}
				containerStats[id] = initialStats
			}
			stats, err := ctr.GetContainerStats(containerStats[id])
			if err != nil && errors.Cause(err) != define.ErrNoSuchCtr {
				return err
			}
			// replace the previous measurement with the current one
			containerStats[id] = stats
			reportStats = append(reportStats, stats)
		}
		ctrs, err = containerFunc()
		if err != nil {
			return err
		}
		if strings.ToLower(format) != formats.JSONString && !c.NoReset {
			tm.Clear()
			tm.MoveCursor(1, 1)
			tm.Flush()
		}
		if err := outputStats(reportStats, format); err != nil {
			return err
		}
		time.Sleep(time.Second)
	}
	return nil
}

func outputStats(stats []*libpod.ContainerStats, format string) error {
	var out formats.Writer
	var outputStats []statsOutputParams
	for _, s := range stats {
		outputStats = append(outputStats, getStatsOutputParams(s))
	}
	if strings.ToLower(format) == formats.JSONString {
		out = formats.JSONStructArray{Output: statsToGeneric(outputStats, []statsOutputParams{})}
	} else {
		var mapOfHeaders map[string]string
		if len(outputStats) == 0 {
			params := getStatsOutputParamsEmpty()
			mapOfHeaders = params.headerMap()
		} else {
			mapOfHeaders = outputStats[0].headerMap()
		}
		out = formats.StdoutTemplateArray{Output: statsToGeneric(outputStats, []statsOutputParams{}), Template: format, Fields: mapOfHeaders}
	}
	return out.Out()
}

func genStatsFormat(format string) string {
	if format != "" {
		// "\t" from the command line is not being recognized as a tab
		// replacing the string "\t" to a tab character if the user passes in "\t"
		return strings.Replace(format, `\t`, "\t", -1)
	}
	return "table {{.ID}}\t{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}\t{{.NetIO}}\t{{.BlockIO}}\t{{.PIDS}}"
}

// imagesToGeneric creates an empty array of interfaces for output
func statsToGeneric(templParams []statsOutputParams, JSONParams []statsOutputParams) (genericParams []interface{}) {
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

// generate the header based on the template provided
func (i *statsOutputParams) headerMap() map[string]string {
	v := reflect.Indirect(reflect.ValueOf(i))
	values := make(map[string]string)

	for i := 0; i < v.NumField(); i++ {
		key := v.Type().Field(i).Name
		value := key
		switch value {
		case "CPUPerc":
			value = "CPU%"
		case "MemUsage":
			value = "MemUsage/Limit"
		case "MemPerc":
			value = "Mem%"
		}
		values[key] = strings.ToUpper(splitCamelCase(value))
	}
	return values
}

func combineHumanValues(a, b uint64) string {
	if a == 0 && b == 0 {
		return "-- / --"
	}
	return fmt.Sprintf("%s / %s", units.HumanSize(float64(a)), units.HumanSize(float64(b)))
}

func floatToPercentString(f float64) string {
	strippedFloat, err := libpod.RemoveScientificNotationFromFloat(f)
	if err != nil || strippedFloat == 0 {
		// If things go bazinga, return a safe value
		return "--"
	}
	return fmt.Sprintf("%.2f", strippedFloat) + "%"
}

func pidsToString(pid uint64) string {
	if pid == 0 {
		// If things go bazinga, return a safe value
		return "--"
	}
	return fmt.Sprintf("%d", pid)
}

func getStatsOutputParams(stats *libpod.ContainerStats) statsOutputParams {
	return statsOutputParams{
		Name:     stats.Name,
		ID:       shortID(stats.ContainerID),
		CPUPerc:  floatToPercentString(stats.CPU),
		MemUsage: combineHumanValues(stats.MemUsage, stats.MemLimit),
		MemPerc:  floatToPercentString(stats.MemPerc),
		NetIO:    combineHumanValues(stats.NetInput, stats.NetOutput),
		BlockIO:  combineHumanValues(stats.BlockInput, stats.BlockOutput),
		PIDS:     pidsToString(stats.PIDs),
	}
}

func getStatsOutputParamsEmpty() statsOutputParams {
	return statsOutputParams{
		Name:     "",
		ID:       "",
		CPUPerc:  "",
		MemUsage: "",
		MemPerc:  "",
		NetIO:    "",
		BlockIO:  "",
		PIDS:     "",
	}
}
