package main

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	tm "github.com/buger/goterm"
	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/cmd/kpod/formats"
	"github.com/projectatomic/libpod/libpod"
	"github.com/urfave/cli"
)

type statsOutputParams struct {
	Container string `json:"name"`
	ID        string `json:"id"`
	CPUPerc   string `json:"cpu_percent"`
	MemUsage  string `json:"mem_usage"`
	MemPerc   string `json:"mem_percent"`
	NetIO     string `json:"netio"`
	BlockIO   string `json:"blocki"`
	PIDS      uint64 `json:"pids"`
}

var (
	statsFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "show all containers. Only running containers are shown by default. The default is false",
		},
		cli.BoolFlag{
			Name:  "no-stream",
			Usage: "disable streaming stats and only pull the first result, default setting is false",
		},
		cli.StringFlag{
			Name:  "format",
			Usage: "pretty-print container statistics using a Go template",
		},
		cli.BoolFlag{
			Name:  "no-reset",
			Usage: "disable resetting the screen between intervals",
		},
	}

	statsDescription = "display a live stream of one or more containers' resource usage statistics"
	statsCommand     = cli.Command{
		Name:        "stats",
		Usage:       "Display percentage of CPU, memory, network I/O, block I/O and PIDs for one or more containers",
		Description: statsDescription,
		Flags:       statsFlags,
		Action:      statsCmd,
		ArgsUsage:   "",
	}
)

func statsCmd(c *cli.Context) error {
	if err := validateFlags(c, statsFlags); err != nil {
		return err
	}

	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	times := -1
	if c.Bool("no-stream") {
		times = 1
	}

	var format string
	var ctrs []*libpod.Container
	var containerFunc func() ([]*libpod.Container, error)
	all := c.Bool("all")

	if c.IsSet("format") {
		format = c.String("format")
	} else {
		format = genStatsFormat()
	}

	if len(c.Args()) > 0 {
		containerFunc = func() ([]*libpod.Container, error) { return runtime.GetContainersByList(c.Args()) }
	} else if all {
		containerFunc = runtime.GetAllContainers
	} else {
		containerFunc = runtime.GetRunningContainers
	}

	ctrs, err = containerFunc()
	if err != nil {
		return errors.Wrapf(err, "unable to get list of containers")
	}

	containerStats := map[string]*libpod.ContainerStats{}
	for _, ctr := range ctrs {
		initialStats, err := ctr.GetContainerStats(&libpod.ContainerStats{})
		if err != nil {
			return err
		}
		containerStats[ctr.ID()] = initialStats
	}
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
				initialStats, err := ctr.GetContainerStats(&libpod.ContainerStats{})
				if err != nil {
					return err
				}
				containerStats[id] = initialStats
			}
			stats, err := ctr.GetContainerStats(containerStats[id])
			if err != nil {
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
		if strings.ToLower(format) != formats.JSONString && !c.Bool("no-reset") {
			tm.Clear()
			tm.MoveCursor(1, 1)
			tm.Flush()
		}
		outputStats(reportStats, format)
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
	if len(outputStats) == 0 {
		return nil
	}
	if strings.ToLower(format) == formats.JSONString {
		out = formats.JSONStructArray{Output: statsToGeneric(outputStats, []statsOutputParams{})}
	} else {
		out = formats.StdoutTemplateArray{Output: statsToGeneric(outputStats, []statsOutputParams{}), Template: format, Fields: outputStats[0].headerMap()}
	}
	return formats.Writer(out).Out()
}

func genStatsFormat() (format string) {
	return "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}\t{{.NetIO}}\t{{.BlockIO}}\t{{.PIDS}}"
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
	return fmt.Sprintf("%s / %s", units.HumanSize(float64(a)), units.HumanSize(float64(b)))
}

func floatToPercentString(f float64) string {
	strippedFloat, err := libpod.RemoveScientificNotationFromFloat(f)
	if err != nil {
		// If things go bazinga, return a safe value
		return "0.00 %"
	}
	return fmt.Sprintf("%.2f", strippedFloat) + "%"
}

func getStatsOutputParams(stats *libpod.ContainerStats) statsOutputParams {
	return statsOutputParams{
		Container: stats.ContainerID[:12],
		ID:        stats.ContainerID,
		CPUPerc:   floatToPercentString(stats.CPU),
		MemUsage:  combineHumanValues(stats.MemUsage, stats.MemLimit),
		MemPerc:   floatToPercentString(stats.MemPerc),
		NetIO:     combineHumanValues(stats.NetInput, stats.NetOutput),
		BlockIO:   combineHumanValues(stats.BlockInput, stats.BlockOutput),
		PIDS:      stats.PIDs,
	}
}
