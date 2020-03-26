package entities

import "time"

type PodKillOptions struct {
	All    bool
	Latest bool
	Signal string
}

type PodKillReport struct {
	Errs []error
	Id   string
}

type ListPodsReport struct {
	Cgroup     string
	Containers []*ListPodContainer
	Created    time.Time
	Id         string
	Name       string
	Namespace  string
	Status     string
}

type ListPodContainer struct {
	Id     string
	Names  string
	Status string
}

type PodPauseOptions struct {
	All    bool
	Latest bool
}

type PodPauseReport struct {
	Errs []error
	Id   string
}

type PodunpauseOptions struct {
	All    bool
	Latest bool
}

type PodUnpauseReport struct {
	Errs []error
	Id   string
}

type PodStopOptions struct {
	All     bool
	Ignore  bool
	Latest  bool
	Timeout int
}

type PodStopReport struct {
	Errs []error
	Id   string
}

type PodRestartOptions struct {
	All    bool
	Latest bool
}

type PodRestartReport struct {
	Errs []error
	Id   string
}

type PodStartOptions struct {
	All    bool
	Latest bool
}

type PodStartReport struct {
	Errs []error
	Id   string
}

type PodRmOptions struct {
	All    bool
	Force  bool
	Ignore bool
	Latest bool
}

type PodRmReport struct {
	Err error
	Id  string
}
