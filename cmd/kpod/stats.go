package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/docker/go-units"

	tm "github.com/buger/goterm"
	"github.com/kubernetes-incubator/cri-o/libkpod"
	"github.com/kubernetes-incubator/cri-o/oci"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var printf func(format string, a ...interface{}) (n int, err error)
var println func(a ...interface{}) (n int, err error)

type statsOutputParams struct {
	Container string
	ID        string
	CPUPerc   string
	MemUsage  string
	MemPerc   string
	NetIO     string
	BlockIO   string
	PIDs      uint64
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
			Name:  "json",
			Usage: "output container statistics in json format",
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
	config, err := getConfig(c)
	if err != nil {
		return errors.Wrapf(err, "could not read config")
	}
	containerServer, err := libkpod.New(config)
	if err != nil {
		return errors.Wrapf(err, "could not create container server")
	}
	defer containerServer.Shutdown()
	err = containerServer.Update()
	if err != nil {
		return errors.Wrapf(err, "could not update list of containers")
	}
	times := -1
	if c.Bool("no-stream") {
		times = 1
	}
	statsChan := make(chan []*libkpod.ContainerStats)
	// iterate over the channel until it is closed
	go func() {
		// print using goterm
		printf = tm.Printf
		println = tm.Println
		for stats := range statsChan {
			// Continually refresh statistics
			tm.Clear()
			tm.MoveCursor(1, 1)
			outputStats(stats, c.String("format"), c.Bool("json"))
			tm.Flush()
			time.Sleep(time.Second)
		}
	}()
	return getStats(containerServer, c.Args(), c.Bool("all"), statsChan, times)
}

func getStats(server *libkpod.ContainerServer, args []string, all bool, statsChan chan []*libkpod.ContainerStats, times int) error {
	ctrs, err := server.ListContainers(isRunning, ctrInList(args))
	if err != nil {
		return err
	}
	containerStats := map[string]*libkpod.ContainerStats{}
	for _, ctr := range ctrs {
		initialStats, err := server.GetContainerStats(ctr, &libkpod.ContainerStats{})
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
		reportStats := []*libkpod.ContainerStats{}
		for _, ctr := range ctrs {
			id := ctr.ID()
			if _, ok := containerStats[ctr.ID()]; !ok {
				initialStats, err := server.GetContainerStats(ctr, &libkpod.ContainerStats{})
				if err != nil {
					return err
				}
				containerStats[id] = initialStats
			}
			stats, err := server.GetContainerStats(ctr, containerStats[id])
			if err != nil {
				return err
			}
			// replace the previous measurement with the current one
			containerStats[id] = stats
			reportStats = append(reportStats, stats)
		}
		statsChan <- reportStats

		err := server.Update()
		if err != nil {
			return err
		}
		ctrs, err = server.ListContainers(isRunning, ctrInList(args))
		if err != nil {
			return err
		}
	}
	return nil
}

func outputStats(stats []*libkpod.ContainerStats, format string, json bool) error {
	if format == "" {
		outputStatsHeader()
	}
	if json {
		return outputStatsAsJSON(stats)
	}
	var err error
	for _, s := range stats {
		if format == "" {
			outputStatsUsingFormatString(s)
		} else {
			params := getStatsOutputParams(s)
			err2 := outputStatsUsingTemplate(format, params)
			if err2 != nil {
				err = errors.Wrapf(err, err2.Error())
			}
		}
	}
	return err
}

func outputStatsHeader() {
	printf("%-64s %-16s %-32s %-16s %-24s %-24s %s\n", "CONTAINER", "CPU %", "MEM USAGE / MEM LIMIT", "MEM %", "NET I/O", "BLOCK I/O", "PIDS")
}

func outputStatsUsingFormatString(stats *libkpod.ContainerStats) {
	printf("%-64s %-16s %-32s %-16s %-24s %-24s %d\n", stats.Container, floatToPercentString(stats.CPU), combineHumanValues(stats.MemUsage, stats.MemLimit), floatToPercentString(stats.MemPerc), combineHumanValues(stats.NetInput, stats.NetOutput), combineHumanValues(stats.BlockInput, stats.BlockOutput), stats.PIDs)
}

func combineHumanValues(a, b uint64) string {
	return fmt.Sprintf("%s / %s", units.HumanSize(float64(a)), units.HumanSize(float64(b)))
}

func floatToPercentString(f float64) string {
	return fmt.Sprintf("%.2f %s", f, "%")
}

func getStatsOutputParams(stats *libkpod.ContainerStats) statsOutputParams {
	return statsOutputParams{
		Container: stats.Container,
		ID:        stats.Container,
		CPUPerc:   floatToPercentString(stats.CPU),
		MemUsage:  combineHumanValues(stats.MemUsage, stats.MemLimit),
		MemPerc:   floatToPercentString(stats.MemPerc),
		NetIO:     combineHumanValues(stats.NetInput, stats.NetOutput),
		BlockIO:   combineHumanValues(stats.BlockInput, stats.BlockOutput),
		PIDs:      stats.PIDs,
	}
}

func outputStatsUsingTemplate(format string, params statsOutputParams) error {
	tmpl, err := template.New("stats").Parse(format)
	if err != nil {
		return errors.Wrapf(err, "template parsing error")
	}

	err = tmpl.Execute(os.Stdout, params)
	if err != nil {
		return err
	}
	println()
	return nil
}

func outputStatsAsJSON(stats []*libkpod.ContainerStats) error {
	s, err := json.Marshal(stats)
	if err != nil {
		return err
	}
	println(s)
	return nil
}

func isRunning(ctr *oci.Container) bool {
	return ctr.State().Status == "running"
}

func ctrInList(idsOrNames []string) func(ctr *oci.Container) bool {
	if len(idsOrNames) == 0 {
		return func(*oci.Container) bool { return true }
	}
	return func(ctr *oci.Container) bool {
		for _, idOrName := range idsOrNames {
			if strings.HasPrefix(ctr.ID(), idOrName) || strings.HasSuffix(ctr.Name(), idOrName) {
				return true
			}
		}
		return false
	}
}
