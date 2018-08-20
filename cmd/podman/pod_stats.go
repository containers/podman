package main

import (
	"fmt"
	"strings"
	"time"

	"encoding/json"
	tm "github.com/buger/goterm"
	"github.com/containers/libpod/cmd/podman/formats"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/ulule/deepcopier"
	"github.com/urfave/cli"
)

var (
	podStatsFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "show stats for all pods.  Only running pods are shown by default.",
		},
		cli.BoolFlag{
			Name:  "no-stream",
			Usage: "disable streaming stats and only pull the first result, default setting is false",
		},
		cli.BoolFlag{
			Name:  "no-reset",
			Usage: "disable resetting the screen between intervals",
		},
		cli.StringFlag{
			Name:  "format",
			Usage: "pretty-print container statistics to JSON or using a Go template",
		}, LatestPodFlag,
	}
	podStatsDescription = "display a live stream of resource usage statistics for the containers in or more pods"
	podStatsCommand     = cli.Command{
		Name:                   "stats",
		Usage:                  "Display percentage of CPU, memory, network I/O, block I/O and PIDs for containers in one or more pods",
		Description:            podStatsDescription,
		Flags:                  podStatsFlags,
		Action:                 podStatsCmd,
		ArgsUsage:              "[POD_NAME_OR_ID]",
		UseShortOptionHandling: true,
	}
)

func podStatsCmd(c *cli.Context) error {
	var (
		podFunc func() ([]*libpod.Pod, error)
	)

	format := c.String("format")
	all := c.Bool("all")
	latest := c.Bool("latest")
	ctr := 0
	if all {
		ctr += 1
	}
	if latest {
		ctr += 1
	}
	if len(c.Args()) > 0 {
		ctr += 1
	}

	if ctr > 1 {
		return errors.Errorf("--all, --latest and containers cannot be used together")
	} else if ctr == 0 {
		// If user didn't specify, imply --all
		all = true
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	times := -1
	if c.Bool("no-stream") {
		times = 1
	}

	if len(c.Args()) > 0 {
		podFunc = func() ([]*libpod.Pod, error) { return getPodsByList(c.Args(), runtime) }
	} else if latest {
		podFunc = func() ([]*libpod.Pod, error) {
			latestPod, err := runtime.GetLatestPod()
			if err != nil {
				return nil, err
			}
			return []*libpod.Pod{latestPod}, err
		}
	} else if all {
		podFunc = runtime.GetAllPods
	} else {
		podFunc = runtime.GetRunningPods
	}

	pods, err := podFunc()
	if err != nil {
		return errors.Wrapf(err, "unable to get a list of pods")
	}

	// First we need to get an initial pass of pod/ctr stats (these are not printed)
	var podStats []*libpod.PodContainerStats
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
		ps := libpod.PodContainerStats{
			Pod:            p,
			ContainerStats: emptyStats,
		}
		podStats = append(podStats, &ps)
	}

	// Create empty container stat results for our first pass
	var previousPodStats []*libpod.PodContainerStats
	for _, p := range pods {
		cs := make(map[string]*libpod.ContainerStats)
		pcs := libpod.PodContainerStats{
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

	for i := 0; i < times; i += step {
		var newStats []*libpod.PodContainerStats
		for _, p := range pods {
			prevStat := getPreviousPodContainerStats(p.ID(), previousPodStats)
			newPodStats, err := p.GetPodStats(prevStat)
			if errors.Cause(err) == libpod.ErrNoSuchPod {
				continue
			}
			if err != nil {
				return err
			}
			newPod := libpod.PodContainerStats{
				Pod:            p,
				ContainerStats: newPodStats,
			}
			newStats = append(newStats, &newPod)
		}
		//Output
		if strings.ToLower(format) != formats.JSONString && !c.Bool("no-reset") {
			tm.Clear()
			tm.MoveCursor(1, 1)
			tm.Flush()
		}
		if strings.ToLower(format) == formats.JSONString {
			outputJson(newStats)

		} else {
			outputToStdOut(newStats)
		}
		time.Sleep(time.Second)
		previousPodStats := new([]*libpod.PodContainerStats)
		deepcopier.Copy(newStats).To(previousPodStats)
		pods, err = podFunc()
		if err != nil {
			return err
		}
	}

	return nil
}

func outputToStdOut(stats []*libpod.PodContainerStats) {
	outFormat := ("%-14s %-14s %-12s %-6s %-19s %-6s %-19s %-19s %-4s\n")
	fmt.Printf(outFormat, "POD", "CID", "NAME", "CPU %", "MEM USAGE/ LIMIT", "MEM %", "NET IO", "BLOCK IO", "PIDS")
	for _, i := range stats {
		if len(i.ContainerStats) == 0 {
			fmt.Printf(outFormat, i.Pod.ID()[:12], "--", "--", "--", "--", "--", "--", "--", "--")
		}
		for _, c := range i.ContainerStats {
			cpu := floatToPercentString(c.CPU)
			memUsage := combineHumanValues(c.MemUsage, c.MemLimit)
			memPerc := floatToPercentString(c.MemPerc)
			netIO := combineHumanValues(c.NetInput, c.NetOutput)
			blockIO := combineHumanValues(c.BlockInput, c.BlockOutput)
			pids := pidsToString(c.PIDs)
			containerName := c.Name
			if len(c.Name) > 10 {
				containerName = containerName[:10]
			}
			fmt.Printf(outFormat, i.Pod.ID()[:12], c.ContainerID[:12], containerName, cpu, memUsage, memPerc, netIO, blockIO, pids)
		}
	}
	fmt.Println()
}

func getPreviousPodContainerStats(podID string, prev []*libpod.PodContainerStats) map[string]*libpod.ContainerStats {
	for _, p := range prev {
		if podID == p.Pod.ID() {
			return p.ContainerStats
		}
	}
	return map[string]*libpod.ContainerStats{}
}

func outputJson(stats []*libpod.PodContainerStats) error {
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
