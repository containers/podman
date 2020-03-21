package entities

import (
	"time"

	"github.com/containers/libpod/libpod/define"
)

type WaitOptions struct {
	Condition define.ContainerStatus
	Interval  time.Duration
	Latest    bool
}

type WaitReport struct {
	Id       string
	Error    error
	ExitCode int32
}

type BoolReport struct {
	Value bool
}

type PauseUnPauseOptions struct {
	All bool
}

type PauseUnpauseReport struct {
	Err error
	Id  string
}

type StopOptions struct {
	All      bool
	CIDFiles []string
	Ignore   bool
	Latest   bool
	Timeout  uint
}

type StopReport struct {
	Err error
	Id  string
}

type KillOptions struct {
	All    bool
	Latest bool
	Signal string
}

type KillReport struct {
	Err error
	Id  string
}

type RestartOptions struct {
	All     bool
	Latest  bool
	Running bool
	Timeout *uint
}

type RestartReport struct {
	Err error
	Id  string
}

type RmOptions struct {
	All      bool
	CIDFiles []string
	Force    bool
	Ignore   bool
	Latest   bool
	Storage  bool
	Volumes  bool
}

type RmReport struct {
	Err error
	Id  string
}
