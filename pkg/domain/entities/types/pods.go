package types

import (
	"time"

	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/specgen"
)

type PodPruneReport struct {
	Err error
	Id  string //nolint:revive,stylecheck
}

type PodPauseReport struct {
	Errs []error
	Id   string //nolint:revive,stylecheck
}
type PodUnpauseReport struct {
	Errs []error
	Id   string //nolint:revive,stylecheck
}

type PodStopReport struct {
	Errs     []error
	Id       string //nolint:revive,stylecheck
	RawInput string
}

type PodRestartReport struct {
	Errs []error
	Id   string //nolint:revive,stylecheck
}

type PodStartReport struct {
	Errs     []error
	Id       string //nolint:revive,stylecheck
	RawInput string
}

type PodRmReport struct {
	RemovedCtrs map[string]error
	Err         error
	Id          string //nolint:revive,stylecheck
}

type PodCreateReport struct {
	Id string //nolint:revive,stylecheck
}

type PodCloneReport struct {
	Id string //nolint:revive,stylecheck
}

// PodStatsReport includes pod-resource statistics data.
type PodStatsReport struct {
	// Percentage of CPU utilized by pod
	// example: 75.5%
	CPU string
	// Humanized Memory usage and maximum
	// example: 12mb / 24mb
	MemUsage string
	// Memory usage and maximum in bytes
	// example: 1,000,000 / 4,000,000
	MemUsageBytes string
	// Percentage of Memory utilized by pod
	// example: 50.5%
	Mem string
	// Network usage inbound + outbound
	NetIO string
	// Humanized disk usage read + write
	BlockIO string
	// Container PID
	PIDS string
	// Pod ID
	// example: 62310217a19e
	Pod string
	// Container ID
	// example: e43534f89a7d
	CID string
	// Pod Name
	// example: elastic_pascal
	Name string
}

// PodSpec is an abstracted version of PodSpecGen designed to eventually accept options
// not meant to be in a specgen
type PodSpec struct {
	PodSpecGen specgen.PodSpecGenerator
}

type PodInspectReport struct {
	*define.InspectPodData
}

type PodKillReport struct {
	Errs []error
	Id   string //nolint:revive,stylecheck
}

type ListPodsReport struct {
	Cgroup     string
	Containers []*ListPodContainer
	Created    time.Time
	Id         string //nolint:revive,stylecheck
	InfraId    string //nolint:revive,stylecheck
	Name       string
	Namespace  string
	// Network names connected to infra container
	Networks []string
	Status   string
	Labels   map[string]string
}

type ListPodContainer struct {
	Id           string //nolint:revive,stylecheck
	Names        string
	Status       string
	RestartCount uint
}
