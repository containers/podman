package types

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
	Errs []error
	Id   string //nolint:revive,stylecheck
}

type PodRestartReport struct {
	Errs []error
	Id   string //nolint:revive,stylecheck
}

type PodStartReport struct {
	Errs []error
	Id   string //nolint:revive,stylecheck
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
