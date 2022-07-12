package abi

import (
	"context"
	"errors"
	"fmt"

	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/utils"
	"github.com/docker/go-units"
)

// PodStats implements printing stats about pods.
func (ic *ContainerEngine) PodStats(ctx context.Context, namesOrIds []string, options entities.PodStatsOptions) ([]*entities.PodStatsReport, error) {
	// Cgroups v2 check for rootless.
	if rootless.IsRootless() {
		unified, err := cgroups.IsCgroup2UnifiedMode()
		if err != nil {
			return nil, err
		}
		if !unified {
			return nil, errors.New("pod stats is not supported in rootless mode without cgroups v2")
		}
	}
	// Get the (running) pods and convert them to the entities format.
	pods, err := getPodsByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, fmt.Errorf("unable to get list of pods: %w", err)
	}
	return ic.podsToStatsReport(pods)
}

// podsToStatsReport converts a slice of pods into a corresponding slice of stats reports.
func (ic *ContainerEngine) podsToStatsReport(pods []*libpod.Pod) ([]*entities.PodStatsReport, error) {
	reports := []*entities.PodStatsReport{}
	for i := range pods { // Access by index to prevent potential loop-variable leaks.
		podStats, err := pods[i].GetPodStats(nil)
		if err != nil {
			return nil, err
		}
		podID := pods[i].ID()[:12]
		for j := range podStats {
			r := entities.PodStatsReport{
				CPU:           floatToPercentString(podStats[j].CPU),
				MemUsage:      combineHumanValues(podStats[j].MemUsage, podStats[j].MemLimit),
				MemUsageBytes: combineBytesValues(podStats[j].MemUsage, podStats[j].MemLimit),
				Mem:           floatToPercentString(podStats[j].MemPerc),
				NetIO:         combineHumanValues(podStats[j].NetInput, podStats[j].NetOutput),
				BlockIO:       combineHumanValues(podStats[j].BlockInput, podStats[j].BlockOutput),
				PIDS:          pidsToString(podStats[j].PIDs),
				CID:           podStats[j].ContainerID[:12],
				Name:          podStats[j].Name,
				Pod:           podID,
			}
			reports = append(reports, &r)
		}
	}

	return reports, nil
}

func combineHumanValues(a, b uint64) string {
	if a == 0 && b == 0 {
		return "-- / --"
	}
	return fmt.Sprintf("%s / %s", units.HumanSize(float64(a)), units.HumanSize(float64(b)))
}

func combineBytesValues(a, b uint64) string {
	if a == 0 && b == 0 {
		return "-- / --"
	}
	return fmt.Sprintf("%s / %s", units.BytesSize(float64(a)), units.BytesSize(float64(b)))
}

func floatToPercentString(f float64) string {
	strippedFloat, err := utils.RemoveScientificNotationFromFloat(f)
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
