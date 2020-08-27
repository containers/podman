package libpod

import (
	"net/http"

	"github.com/containers/podman/v2/libpod/define"
	"k8s.io/client-go/tools/remotecommand"
)

// OCIRuntime is an implementation of an OCI runtime.
// The OCI runtime implementation is expected to be a fairly thin wrapper around
// the actual runtime, and is not expected to include things like state
// management logic - e.g., we do not expect it to determine on its own that
// calling 'UnpauseContainer()' on a container that is not paused is an error.
// The code calling the OCIRuntime will manage this.
// TODO: May want to move the Attach() code under this umbrella. It's highly OCI
// runtime dependent.
// TODO: May want to move the conmon cleanup code here too - it depends on
// Conmon being in use.
type OCIRuntime interface {
	// Name returns the name of the runtime.
	Name() string
	// Path returns the path to the runtime executable.
	Path() string

	// CreateContainer creates the container in the OCI runtime.
	CreateContainer(ctr *Container, restoreOptions *ContainerCheckpointOptions) error
	// UpdateContainerStatus updates the status of the given container.
	UpdateContainerStatus(ctr *Container) error
	// StartContainer starts the given container.
	StartContainer(ctr *Container) error
	// KillContainer sends the given signal to the given container.
	// If all is set, all processes in the container will be signalled;
	// otherwise, only init will be signalled.
	KillContainer(ctr *Container, signal uint, all bool) error
	// StopContainer stops the given container.
	// The container's stop signal (or SIGTERM if unspecified) will be sent
	// first.
	// After the given timeout, SIGKILL will be sent.
	// If the given timeout is 0, SIGKILL will be sent immediately, and the
	// stop signal will be omitted.
	// If all is set, we will attempt to use the --all flag will `kill` in
	// the OCI runtime to kill all processes in the container, including
	// exec sessions. This is only supported if the container has cgroups.
	StopContainer(ctr *Container, timeout uint, all bool) error
	// DeleteContainer deletes the given container from the OCI runtime.
	DeleteContainer(ctr *Container) error
	// PauseContainer pauses the given container.
	PauseContainer(ctr *Container) error
	// UnpauseContainer unpauses the given container.
	UnpauseContainer(ctr *Container) error

	// HTTPAttach performs an attach intended to be transported over HTTP.
	// For terminal attach, the container's output will be directly streamed
	// to output; otherwise, STDOUT and STDERR will be multiplexed, with
	// a header prepended as follows: 1-byte STREAM (0, 1, 2 for STDIN,
	// STDOUT, STDERR), 3 null (0x00) bytes, 4-byte big endian length.
	// If a cancel channel is provided, it can be used to asynchronously
	// termninate the attach session. Detach keys, if given, will also cause
	// the attach session to be terminated if provided via the STDIN
	// channel. If they are not provided, the default detach keys will be
	// used instead. Detach keys of "" will disable detaching via keyboard.
	// The streams parameter will determine which streams to forward to the
	// client.
	HTTPAttach(ctr *Container, r *http.Request, w http.ResponseWriter, streams *HTTPAttachStreams, detachKeys *string, cancel <-chan bool, hijackDone chan<- bool, streamAttach, streamLogs bool) error
	// AttachResize resizes the terminal in use by the given container.
	AttachResize(ctr *Container, newSize remotecommand.TerminalSize) error

	// ExecContainer executes a command in a running container.
	// Returns an int (PID of exec session), error channel (errors from
	// attach), and error (errors that occurred attempting to start the exec
	// session). This returns once the exec session is running - not once it
	// has completed, as one might expect. The attach session will remain
	// running, in a goroutine that will return via the chan error in the
	// return signature.
	ExecContainer(ctr *Container, sessionID string, options *ExecOptions, streams *define.AttachStreams) (int, chan error, error)
	// ExecContainerHTTP executes a command in a running container and
	// attaches its standard streams to a provided hijacked HTTP session.
	// Maintains the same invariants as ExecContainer (returns on session
	// start, with a goroutine running in the background to handle attach).
	// The HTTP attach itself maintains the same invariants as HTTPAttach.
	ExecContainerHTTP(ctr *Container, sessionID string, options *ExecOptions, r *http.Request, w http.ResponseWriter, streams *HTTPAttachStreams, cancel <-chan bool, hijackDone chan<- bool, holdConnOpen <-chan bool) (int, chan error, error)
	// ExecContainerDetached executes a command in a running container, but
	// does not attach to it. Returns the PID of the exec session and an
	// error (if starting the exec session failed)
	ExecContainerDetached(ctr *Container, sessionID string, options *ExecOptions, stdin bool) (int, error)
	// ExecAttachResize resizes the terminal of a running exec session. Only
	// allowed with sessions that were created with a TTY.
	ExecAttachResize(ctr *Container, sessionID string, newSize remotecommand.TerminalSize) error
	// ExecStopContainer stops a given exec session in a running container.
	// SIGTERM with be sent initially, then SIGKILL after the given timeout.
	// If timeout is 0, SIGKILL will be sent immediately, and SIGTERM will
	// be omitted.
	ExecStopContainer(ctr *Container, sessionID string, timeout uint) error
	// ExecUpdateStatus checks the status of a given exec session.
	// Returns true if the session is still running, or false if it exited.
	ExecUpdateStatus(ctr *Container, sessionID string) (bool, error)
	// ExecContainerCleanup cleans up after an exec session exits.
	// It removes any files left by the exec session that are no longer
	// needed, including the attach socket.
	ExecContainerCleanup(ctr *Container, sessionID string) error

	// CheckpointContainer checkpoints the given container.
	// Some OCI runtimes may not support this - if SupportsCheckpoint()
	// returns false, this is not implemented, and will always return an
	// error.
	CheckpointContainer(ctr *Container, options ContainerCheckpointOptions) error

	// CheckConmonRunning verifies that the given container's Conmon
	// instance is still running. Runtimes without Conmon, or systems where
	// the PID of conmon is not available, should mock this as True.
	// True indicates that Conmon for the instance is running, False
	// indicates it is not.
	CheckConmonRunning(ctr *Container) (bool, error)

	// SupportsCheckpoint returns whether this OCI runtime
	// implementation supports the CheckpointContainer() operation.
	SupportsCheckpoint() bool
	// SupportsJSONErrors is whether the runtime can return JSON-formatted
	// error messages.
	SupportsJSONErrors() bool
	// SupportsNoCgroups is whether the runtime supports running containers
	// without cgroups.
	SupportsNoCgroups() bool
	// SupportsKVM os whether the OCI runtime supports running containers
	// without KVM separation
	SupportsKVM() bool

	// AttachSocketPath is the path to the socket to attach to a given
	// container.
	// TODO: If we move Attach code in here, this should be made internal.
	// We don't want to force all runtimes to share the same attach
	// implementation.
	AttachSocketPath(ctr *Container) (string, error)
	// ExecAttachSocketPath is the path to the socket to attach to a given
	// exec session in the given container.
	// TODO: Probably should be made internal.
	ExecAttachSocketPath(ctr *Container, sessionID string) (string, error)
	// ExitFilePath is the path to a container's exit file.
	// All runtime implementations must create an exit file when containers
	// exit, containing the exit code of the container (as a string).
	// This is the path to that file for a given container.
	ExitFilePath(ctr *Container) (string, error)

	// RuntimeInfo returns verbose information about the runtime.
	RuntimeInfo() (*define.ConmonInfo, *define.OCIRuntimeInfo, error)
}

// ExecOptions are options passed into ExecContainer. They control the command
// that will be executed and how the exec will proceed.
type ExecOptions struct {
	// Cmd is the command to execute.
	Cmd []string
	// CapAdd is a set of capabilities to add to the executed command.
	CapAdd []string
	// Env is a set of environment variables to add to the container.
	Env map[string]string
	// Terminal is whether to create a new TTY for the exec session.
	Terminal bool
	// Cwd is the working directory for the executed command. If unset, the
	// working directory of the container will be used.
	Cwd string
	// User is the user the command will be executed as. If unset, the user
	// the container was run as will be used.
	User string
	// Streams are the streams that will be attached to the container.
	Streams *define.AttachStreams
	// PreserveFDs is a number of additional file descriptors (in addition
	// to 0, 1, 2) that will be passed to the executed process. The total FDs
	// passed will be 3 + PreserveFDs.
	PreserveFDs uint
	// DetachKeys is a set of keys that, when pressed in sequence, will
	// detach from the container.
	// If not provided, the default keys will be used.
	// If provided but set to "", detaching from the container will be
	// disabled.
	DetachKeys *string
	// ExitCommand is a command that will be run after the exec session
	// exits.
	ExitCommand []string
	// ExitCommandDelay is a delay (in seconds) between the exec session
	// exiting, and the exit command being invoked.
	ExitCommandDelay uint
}

// HTTPAttachStreams informs the HTTPAttach endpoint which of the container's
// standard streams should be streamed to the client. If this is passed, at
// least one of the streams must be set to true.
type HTTPAttachStreams struct {
	Stdin  bool
	Stdout bool
	Stderr bool
}
