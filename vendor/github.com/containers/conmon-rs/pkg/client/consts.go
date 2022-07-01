package client

const (
	// LogDriverStdout is the log driver printing to stdio.
	LogDriverStdout = "stdout"

	// LogDriverSystemd is the log driver printing to systemd journald.
	LogDriverSystemd = "systemd"

	// LogLevelTrace is the log level printing only "trace" messages.
	LogLevelTrace = "trace"

	// LogLevelDebug is the log level printing only "debug" messages.
	LogLevelDebug = "debug"

	// LogLevelInfo is the log level printing only "info" messages.
	LogLevelInfo = "info"

	// LogLevelWarn is the log level printing only "warn" messages.
	LogLevelWarn = "warn"

	// LogLevelError is the log level printing only "error" messages.
	LogLevelError = "error"

	// LogLevelOff is the log level printing no messages.
	LogLevelOff = "off"
)
