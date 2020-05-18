package entities

import (
	"io"
	"net/url"
	"os"
	"time"

	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/specgen"
	"github.com/cri-o/ocicni/pkg/ocicni"
)

// ContainerRunlabelOptions are the options to execute container-runlabel.
type ContainerRunlabelOptions struct {
	// Authfile - path to an authentication file.
	Authfile string
	// CertDir - path to a directory containing TLS certifications and
	// keys.
	CertDir string
	// Credentials - `user:password` to use when pulling an image.
	Credentials string
	// Display - do not execute but print the command.
	Display bool
	// Replace - replace an existing container with a new one from the
	// image.
	Replace bool
	// Name - use this name when executing the runlabel container.
	Name string
	// Optional1 - fist optional parameter for install.
	Optional1 string
	// Optional2 - second optional parameter for install.
	Optional2 string
	// Optional3 - third optional parameter for install.
	Optional3 string
	// Pull - pull the specified image if it's not in the local storage.
	Pull bool
	// Quiet - suppress output when pulling images.
	Quiet bool
	// SignaturePolicy - path to a signature-policy file.
	SignaturePolicy string
	// SkipTLSVerify - skip HTTPS and certificate verifications when
	// contacting registries.
	SkipTLSVerify types.OptionalBool
}

// ContainerRunlabelReport contains the results from executing container-runlabel.
type ContainerRunlabelReport struct {
}

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
	LeaveRunning   bool
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

// AttachOptions describes the cli and other values
// needed to perform an attach
type AttachOptions struct {
	DetachKeys string
	Latest     bool
	NoStdin    bool
	SigProxy   bool
	Stdin      *os.File
	Stdout     *os.File
	Stderr     *os.File
}

// ContainerLogsOptions describes the options to extract container logs.
type ContainerLogsOptions struct {
	// Show extra details provided to the logs.
	Details bool
	// Follow the log output.
	Follow bool
	// Display logs for the latest container only. Ignored on the remote client.
	Latest bool
	// Show container names in the output.
	Names bool
	// Show logs since this timestamp.
	Since time.Time
	// Number of lines to display at the end of the output.
	Tail int64
	// Show timestamps in the logs.
	Timestamps bool
	// Write the logs to Writer.
	Writer io.Writer
}

// ExecOptions describes the cli values to exec into
// a container
type ExecOptions struct {
	Cmd         []string
	DetachKeys  string
	Envs        map[string]string
	Interactive bool
	Latest      bool
	PreserveFDs uint
	Privileged  bool
	Tty         bool
	User        string
	WorkDir     string
}

// ContainerStartOptions describes the val from the
// CLI needed to start a container
type ContainerStartOptions struct {
	Attach      bool
	DetachKeys  string
	Interactive bool
	Latest      bool
	SigProxy    bool
	Stdout      *os.File
	Stderr      *os.File
	Stdin       *os.File
}

// ContainerStartReport describes the response from starting
// containers from the cli
type ContainerStartReport struct {
	Id       string
	RawInput string
	Err      error
	ExitCode int
}

// ContainerListOptions describes the CLI options
// for listing containers
type ContainerListOptions struct {
	All       bool
	Filters   map[string][]string
	Format    string
	Last      int
	Latest    bool
	Namespace bool
	Pod       bool
	Quiet     bool
	Size      bool
	Sort      string
	Sync      bool
	Watch     uint
}

// ContainerRunOptions describes the options needed
// to run a container from the CLI
type ContainerRunOptions struct {
	Detach       bool
	DetachKeys   string
	ErrorStream  *os.File
	InputStream  *os.File
	OutputStream *os.File
	Rm           bool
	SigProxy     bool
	Spec         *specgen.SpecGenerator
}

// ContainerRunReport describes the results of running
// a container
type ContainerRunReport struct {
	ExitCode int
	Id       string
}

// ContainerCleanupOptions are the CLI values for the
// cleanup command
type ContainerCleanupOptions struct {
	All         bool
	Latest      bool
	Remove      bool
	RemoveImage bool
}

// ContainerCleanupReport describes the response from a
// container cleanup
type ContainerCleanupReport struct {
	CleanErr error
	Id       string
	RmErr    error
	RmiErr   error
}

// ContainerInitOptions describes input options
// for the container init cli
type ContainerInitOptions struct {
	All    bool
	Latest bool
}

// ContainerInitReport describes the results of a
// container init
type ContainerInitReport struct {
	Err error
	Id  string
}

//ContainerMountOptions describes the input values for mounting containers
// in the CLI
type ContainerMountOptions struct {
	All        bool
	Format     string
	Latest     bool
	NoTruncate bool
}

// ContainerUnmountOptions are the options from the cli for unmounting
type ContainerUnmountOptions struct {
	All    bool
	Force  bool
	Latest bool
}

// ContainerMountReport describes the response from container mount
type ContainerMountReport struct {
	Err  error
	Id   string
	Name string
	Path string
}

// ContainerUnmountReport describes the response from umounting a container
type ContainerUnmountReport struct {
	Err error
	Id  string
}

// ContainerPruneOptions describes the options needed
// to prune a container from the CLI
type ContainerPruneOptions struct {
	Filters url.Values `json:"filters" schema:"filters"`
}

// ContainerPruneReport describes the results after pruning the
// stopped containers.
type ContainerPruneReport struct {
	ID  map[string]int64
	Err map[string]error
}

// ContainerPortOptions describes the options to obtain
// port information on containers
type ContainerPortOptions struct {
	All    bool
	Latest bool
}

// ContainerPortReport describes the output needed for
// the CLI to output ports
type ContainerPortReport struct {
	Id    string
	Ports []ocicni.PortMapping
}

// ContainerCpOptions describes input options for cp
type ContainerCpOptions struct {
	Pause   bool
	Extract bool
}

// ContainerCpReport describes the output from a cp operation
type ContainerCpReport struct {
}

// ContainerStatsOptions describes input options for getting
// stats on containers
type ContainerStatsOptions struct {
	All      bool
	Format   string
	Latest   bool
	NoReset  bool
	NoStream bool
	StatChan chan []*define.ContainerStats
}
