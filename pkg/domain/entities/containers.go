package entities

import (
	"io"
	"net/url"
	"os"
	"time"

	nettypes "github.com/containers/common/libnetwork/types"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/containers/storage/pkg/archive"
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
type ContainerRunlabelReport struct{}

type WaitOptions struct {
	Condition []define.ContainerStatus
	Interval  time.Duration
	Latest    bool
}

type WaitReport struct {
	Id       string //nolint:revive,stylecheck
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
	Id  string //nolint:revive,stylecheck
}

type StopOptions struct {
	Filters map[string][]string
	All     bool
	Ignore  bool
	Latest  bool
	Timeout *uint
}

type StopReport struct {
	Err      error
	Id       string //nolint:revive,stylecheck
	RawInput string
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
	Err      error
	Id       string //nolint:revive,stylecheck
	RawInput string
}

type RestartOptions struct {
	All     bool
	Latest  bool
	Running bool
	Timeout *uint
}

type RestartReport struct {
	Err error
	Id  string //nolint:revive,stylecheck
}

type RmOptions struct {
	All     bool
	Depend  bool
	Force   bool
	Ignore  bool
	Latest  bool
	Timeout *uint
	Volumes bool
}

type ContainerInspectReport struct {
	*define.InspectContainerData
}

type ContainerStatReport struct {
	define.FileInfo
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
	Squash         bool
	Writer         io.Writer
}

type CopyOptions struct {
	// If used with ContainerCopyFromArchive and set to true
	// it will change ownership of files from the source tar archive
	// to the primary uid/gid of the destination container.
	Chown bool
	// Map to translate path names.
	Rename map[string]string
	// NoOverwriteDirNonDir when true prevents an existing directory or file from being overwritten
	// by the other type
	NoOverwriteDirNonDir bool
}

type CommitReport struct {
	Id string //nolint:revive,stylecheck
}

type ContainerExportOptions struct {
	Output string
}

type CheckpointOptions struct {
	All            bool
	Export         string
	CreateImage    string
	IgnoreRootFS   bool
	IgnoreVolumes  bool
	Keep           bool
	Latest         bool
	LeaveRunning   bool
	TCPEstablished bool
	PreCheckPoint  bool
	WithPrevious   bool
	Compression    archive.Compression
	PrintStats     bool
	FileLocks      bool
}

type CheckpointReport struct {
	Err             error                                   `json:"-"`
	Id              string                                  `json:"Id"` //nolint:revive,stylecheck
	RuntimeDuration int64                                   `json:"runtime_checkpoint_duration"`
	CRIUStatistics  *define.CRIUCheckpointRestoreStatistics `json:"criu_statistics"`
}

type RestoreOptions struct {
	All             bool
	IgnoreRootFS    bool
	IgnoreVolumes   bool
	IgnoreStaticIP  bool
	IgnoreStaticMAC bool
	Import          string
	CheckpointImage bool
	Keep            bool
	Latest          bool
	Name            string
	TCPEstablished  bool
	ImportPrevious  string
	PublishPorts    []string
	Pod             string
	PrintStats      bool
	FileLocks       bool
}

type RestoreReport struct {
	Err             error                                   `json:"-"`
	Id              string                                  `json:"Id"` //nolint:revive,stylecheck
	RuntimeDuration int64                                   `json:"runtime_restore_duration"`
	CRIUStatistics  *define.CRIUCheckpointRestoreStatistics `json:"criu_statistics"`
}

type ContainerCreateReport struct {
	Id string //nolint:revive,stylecheck
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
	// Show logs until this timestamp.
	Until time.Time
	// Number of lines to display at the end of the output.
	Tail int64
	// Show timestamps in the logs.
	Timestamps bool
	// Show different colors in the logs.
	Colors bool
	// Write the stdout to this Writer.
	StdoutWriter io.Writer
	// Write the stderr to this Writer.
	StderrWriter io.Writer
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

// ContainerExistsOptions describes the cli values to check if a container exists
type ContainerExistsOptions struct {
	External bool
}

// ContainerStartOptions describes the val from the
// CLI needed to start a container
type ContainerStartOptions struct {
	Filters     map[string][]string
	All         bool
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
	Id       string //nolint:revive,stylecheck
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
	External  bool
	Sort      string
	Sync      bool
	Watch     uint
}

// ContainerRunOptions describes the options needed
// to run a container from the CLI
type ContainerRunOptions struct {
	CIDFile      string
	Detach       bool
	DetachKeys   string
	ErrorStream  *os.File
	InputStream  *os.File
	OutputStream *os.File
	PreserveFDs  uint
	Rm           bool
	SigProxy     bool
	Spec         *specgen.SpecGenerator
	Passwd       bool
}

// ContainerRunReport describes the results of running
// a container
type ContainerRunReport struct {
	ExitCode int
	Id       string //nolint:revive,stylecheck
}

// ContainerCleanupOptions are the CLI values for the
// cleanup command
type ContainerCleanupOptions struct {
	All         bool
	Exec        string
	Latest      bool
	Remove      bool
	RemoveImage bool
}

// ContainerCleanupReport describes the response from a
// container cleanup
type ContainerCleanupReport struct {
	CleanErr error
	Id       string //nolint:revive,stylecheck
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
	Id  string //nolint:revive,stylecheck
}

// ContainerMountOptions describes the input values for mounting containers
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
	Id   string //nolint:revive,stylecheck
	Name string
	Path string
}

// ContainerUnmountReport describes the response from umounting a container
type ContainerUnmountReport struct {
	Err error
	Id  string //nolint:revive,stylecheck
}

// ContainerPruneOptions describes the options needed
// to prune a container from the CLI
type ContainerPruneOptions struct {
	Filters url.Values `json:"filters" schema:"filters"`
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
	Id    string //nolint:revive,stylecheck
	Ports []nettypes.PortMapping
}

// ContainerCpOptions describes input options for cp.
type ContainerCpOptions struct {
	// Pause the container while copying.
	Pause bool
	// Extract the tarfile into the destination directory.
	Extract bool
	// OverwriteDirNonDir allows for overwriting a directory with a
	// non-directory and vice versa.
	OverwriteDirNonDir bool
}

// ContainerStatsOptions describes input options for getting
// stats on containers
type ContainerStatsOptions struct {
	// Operate on the latest known container.  Only supported for local
	// clients.
	Latest bool
	// Stream stats.
	Stream bool
	// Interval in seconds
	Interval int
}

// ContainerStatsReport is used for streaming container stats.
type ContainerStatsReport struct {
	// Error from reading stats.
	Error error
	// Results, set when there is no error.
	Stats []define.ContainerStats
}

// ContainerRenameOptions describes input options for renaming a container.
type ContainerRenameOptions struct {
	// NewName is the new name that will be given to the container.
	NewName string
}

// ContainerCloneOptions contains options for cloning an existing continer
type ContainerCloneOptions struct {
	ID           string
	Destroy      bool
	CreateOpts   ContainerCreateOptions
	Image        string
	RawImageName string
	Run          bool
	Force        bool
}
