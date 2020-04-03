package entities

import (
	"io"
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

// StringSliceReport wraps a string slice.
type StringSliceReport struct {
	Value []string
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

type TopOptions struct {
	// CLI flags.
	ListDescriptors bool
	Latest          bool

	// Options for the API.
	Descriptors []string
	NameOrID    string
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

type ContainerInspectReport struct {
	*define.InspectContainerData
}

type CommitOptions struct {
	Author         string
	Changes        []string
	Format         string
	ImageName      string
	IncludeVolumes bool
	Message        string
	Pause          bool
	Quiet          bool
	Writer         io.Writer
}

type CommitReport struct {
	Id string
}

type ContainerExportOptions struct {
	Output string
}

type CheckpointOptions struct {
	All            bool
	Export         string
	IgnoreRootFS   bool
	Keep           bool
	Latest         bool
	LeaveRuninng   bool
	TCPEstablished bool
}

type CheckpointReport struct {
	Err error
	Id  string
}

type RestoreOptions struct {
	All             bool
	IgnoreRootFS    bool
	IgnoreStaticIP  bool
	IgnoreStaticMAC bool
	Import          string
	Keep            bool
	Latest          bool
	Name            string
	TCPEstablished  bool
}

type RestoreReport struct {
	Err error
	Id  string
}

type ContainerCreateReport struct {
	Id string
}
