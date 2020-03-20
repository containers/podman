package define

// InspectExecSession contains information about a given exec session.
type InspectExecSession struct {
	// CanRemove is legacy and used purely for compatibility reasons.
	// Will always be set to true, unless the exec session is running.
	CanRemove bool `json:"CanRemove"`
	// ContainerID is the ID of the container this exec session is attached
	// to.
	ContainerID string `json:"ContainerID"`
	// DetachKeys are the detach keys used by the exec session.
	// If set to "" the default keys are being used.
	// Will show "<none>" if no detach keys are set.
	DetachKeys string `json:"DetachKeys"`
	// ExitCode is the exit code of the exec session. Will be set to 0 if
	// the exec session has not yet exited.
	ExitCode int `json:"ExitCode"`
	// ID is the ID of the exec session.
	ID string `json:"ID"`
	// OpenStderr is whether the container's STDERR stream will be attached.
	// Always set to true if the exec session created a TTY.
	OpenStderr bool `json:"OpenStderr"`
	// OpenStdin is whether the container's STDIN stream will be attached
	// to.
	OpenStdin bool `json:"OpenStdin"`
	// OpenStdout is whether the container's STDOUT stream will be attached.
	// Always set to true if the exec session created a TTY.
	OpenStdout bool `json:"OpenStdout"`
	// Running is whether the exec session is running.
	Running bool `json:"Running"`
	// Pid is the PID of the exec session's process.
	// Will be set to 0 if the exec session is not running.
	Pid int `json:"Pid"`
	// ProcessConfig contains information about the exec session's process.
	ProcessConfig *InspectExecProcess `json:"ProcessConfig"`
}

// InspectExecProcess contains information about the process in a given exec
// session.
type InspectExecProcess struct {
	// Arguments are the arguments to the entrypoint command of the exec
	// session.
	Arguments []string `json:"arguments"`
	// Entrypoint is the entrypoint for the exec session (the command that
	// will be executed in the container).
	Entrypoint string `json:"entrypoint"`
	// Privileged is whether the exec session will be started with elevated
	// privileges.
	Privileged bool `json:"privileged"`
	// Tty is whether the exec session created a terminal.
	Tty bool `json:"tty"`
	// User is the user the exec session was started as.
	User string `json:"user"`
}
