package client

// LogLevel is the enum for all available server log levels.
type LogLevel string

const (
	// LogLevelTrace is the log level printing only "trace" messages.
	LogLevelTrace LogLevel = "trace"

	// LogLevelDebug is the log level printing only "debug" messages.
	LogLevelDebug LogLevel = "debug"

	// LogLevelInfo is the log level printing only "info" messages.
	LogLevelInfo LogLevel = "info"

	// LogLevelWarn is the log level printing only "warn" messages.
	LogLevelWarn LogLevel = "warn"

	// LogLevelError is the log level printing only "error" messages.
	LogLevelError LogLevel = "error"

	// LogLevelOff is the log level printing no messages.
	LogLevelOff LogLevel = "off"
)

// LogDriver is the enum for all available server log drivers.
type LogDriver string

const (
	// LogDriverStdout is the log driver printing to stdio.
	LogDriverStdout LogDriver = "stdout"

	// LogDriverSystemd is the log driver printing to systemd journald.
	LogDriverSystemd LogDriver = "systemd"
)

// CgroupManager is the enum for all available cgroup managers.
type CgroupManager int

const (
	// CgroupManagerSystemd specifies to use systemd to create and manage
	// cgroups.
	CgroupManagerSystemd CgroupManager = iota

	// CgroupManagerCgroupfs specifies to use the cgroup filesystem to create
	// and manage cgroups.
	CgroupManagerCgroupfs
)
