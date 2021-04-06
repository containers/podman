package containers

import (
	"bufio"
	"io"

	"github.com/containers/podman/v3/libpod/define"
)

//go:generate go run ../generator/generator.go LogOptions
// LogOptions describe finer control of log content or
// how the content is formatted.
type LogOptions struct {
	Follow     *bool
	Since      *string
	Stderr     *bool
	Stdout     *bool
	Tail       *string
	Timestamps *bool
	Until      *string
}

//go:generate go run ../generator/generator.go CommitOptions
// CommitOptions describe details about the resulting committed
// image as defined by repo and tag. None of these options
// are required.
type CommitOptions struct {
	Author  *string
	Changes []string
	Comment *string
	Format  *string
	Pause   *bool
	Repo    *string
	Tag     *string
}

//go:generate go run ../generator/generator.go AttachOptions
// AttachOptions are optional options for attaching to containers
type AttachOptions struct {
	DetachKeys *string
	Logs       *bool
	Stream     *bool
}

//go:generate go run ../generator/generator.go CheckpointOptions
// CheckpointOptions are optional options for checkpointing containers
type CheckpointOptions struct {
	Export         *string
	IgnoreRootfs   *bool
	Keep           *bool
	LeaveRunning   *bool
	TCPEstablished *bool
}

//go:generate go run ../generator/generator.go RestoreOptions
// RestoreOptions are optional options for restoring containers
type RestoreOptions struct {
	IgnoreRootfs    *bool
	IgnoreStaticIP  *bool
	IgnoreStaticMAC *bool
	ImportAchive    *string
	Keep            *bool
	Name            *string
	TCPEstablished  *bool
}

//go:generate go run ../generator/generator.go CreateOptions
// CreateOptions are optional options for creating containers
type CreateOptions struct{}

//go:generate go run ../generator/generator.go DiffOptions
// DiffOptions are optional options for creating containers
type DiffOptions struct{}

//go:generate go run ../generator/generator.go ExecInspectOptions
// ExecInspectOptions are optional options for inspecting
// exec sessions
type ExecInspectOptions struct{}

//go:generate go run ../generator/generator.go ExecStartOptions
// ExecStartOptions are optional options for starting
// exec sessions
type ExecStartOptions struct{}

//go:generate go run ../generator/generator.go HealthCheckOptions
// HealthCheckOptions are optional options for checking
// the health of a container
type HealthCheckOptions struct{}

//go:generate go run ../generator/generator.go MountOptions
// MountOptions are optional options for mounting
// containers
type MountOptions struct{}

//go:generate go run ../generator/generator.go UnmountOptions
// UnmountOptions are optional options for unmounting
// containers
type UnmountOptions struct{}

//go:generate go run ../generator/generator.go MountedContainerPathsOptions
// MountedContainerPathsOptions are optional options for getting
// container mount paths
type MountedContainerPathsOptions struct{}

//go:generate go run ../generator/generator.go ListOptions
// ListOptions are optional options for listing containers
type ListOptions struct {
	All       *bool
	External  *bool
	Filters   map[string][]string
	Last      *int
	Namespace *bool
	Size      *bool
	Sync      *bool
}

//go:generate go run ../generator/generator.go PruneOptions
// PruneOptions are optional options for pruning containers
type PruneOptions struct {
	Filters map[string][]string
}

//go:generate go run ../generator/generator.go RemoveOptions
// RemoveOptions are optional options for removing containers
type RemoveOptions struct {
	Ignore  *bool
	Force   *bool
	Volumes *bool
}

//go:generate go run ../generator/generator.go InspectOptions
// InspectOptions are optional options for inspecting containers
type InspectOptions struct {
	Size *bool
}

//go:generate go run ../generator/generator.go KillOptions
// KillOptions are optional options for killing containers
type KillOptions struct {
	Signal *string
}

//go:generate go run ../generator/generator.go PauseOptions
// PauseOptions are optional options for pausing containers
type PauseOptions struct{}

//go:generate go run ../generator/generator.go RestartOptions
// RestartOptions are optional options for restarting containers
type RestartOptions struct {
	Timeout *int
}

//go:generate go run ../generator/generator.go StartOptions
// StartOptions are optional options for starting containers
type StartOptions struct {
	DetachKeys *string
	Recursive  *bool
}

//go:generate go run ../generator/generator.go StatsOptions
// StatsOptions are optional options for getting stats on containers
type StatsOptions struct {
	Stream *bool
}

//go:generate go run ../generator/generator.go TopOptions
// TopOptions are optional options for getting running
// processes in containers
type TopOptions struct {
	Descriptors *[]string
}

//go:generate go run ../generator/generator.go UnpauseOptions
// UnpauseOptions are optional options for unpausing containers
type UnpauseOptions struct{}

//go:generate go run ../generator/generator.go WaitOptions
// WaitOptions are optional options for waiting on containers
type WaitOptions struct {
	Condition []define.ContainerStatus
	Interval  *string
}

//go:generate go run ../generator/generator.go StopOptions
// StopOptions are optional options for stopping containers
type StopOptions struct {
	Ignore  *bool
	Timeout *uint
}

//go:generate go run ../generator/generator.go ExportOptions
// ExportOptions are optional options for exporting containers
type ExportOptions struct{}

//go:generate go run ../generator/generator.go InitOptions
// InitOptions are optional options for initing containers
type InitOptions struct{}

//go:generate go run ../generator/generator.go ShouldRestartOptions
// ShouldRestartOptions
type ShouldRestartOptions struct{}

//go:generate go run ../generator/generator.go RenameOptions
// RenameOptions are options for renaming containers.
// The Name field is required.
type RenameOptions struct {
	Name *string
}

//go:generate go run ../generator/generator.go ResizeTTYOptions
// ResizeTTYOptions are optional options for resizing
// container TTYs
type ResizeTTYOptions struct {
	Height  *int
	Width   *int
	Running *bool
}

//go:generate go run ../generator/generator.go ResizeExecTTYOptions
// ResizeExecTTYOptions are optional options for resizing
// container ExecTTYs
type ResizeExecTTYOptions struct {
	Height *int
	Width  *int
}

//go:generate go run ../generator/generator.go ExecStartAndAttachOptions
// ExecStartAndAttachOptions are optional options for resizing
// container ExecTTYs
type ExecStartAndAttachOptions struct {
	// OutputStream will be attached to container's STDOUT
	OutputStream *io.WriteCloser
	// ErrorStream will be attached to container's STDERR
	ErrorStream *io.WriteCloser
	// InputStream will be attached to container's STDIN
	InputStream *bufio.Reader
	// AttachOutput is whether to attach to STDOUT
	// If false, stdout will not be attached
	AttachOutput *bool
	// AttachError is whether to attach to STDERR
	// If false, stdout will not be attached
	AttachError *bool
	// AttachInput is whether to attach to STDIN
	// If false, stdout will not be attached
	AttachInput *bool
}

//go:generate go run ../generator/generator.go ExistsOptions
// ExistsOptions are optional options for checking if a container exists
type ExistsOptions struct {
	// External checks for containers created outside of Podman
	External *bool
}
