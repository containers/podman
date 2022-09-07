package define

import (
	"fmt"
	"strings"
)

const (
	// HealthCheckHealthy describes a healthy container
	HealthCheckHealthy string = "healthy"
	// HealthCheckUnhealthy describes an unhealthy container
	HealthCheckUnhealthy string = "unhealthy"
	// HealthCheckStarting describes the time between when the container starts
	// and the start-period (time allowed for the container to start and application
	// to be running) expires.
	HealthCheckStarting string = "starting"
)

// HealthCheckStatus represents the current state of a container
type HealthCheckStatus int

const (
	// HealthCheckSuccess means the health worked
	HealthCheckSuccess HealthCheckStatus = iota
	// HealthCheckFailure means the health ran and failed
	HealthCheckFailure HealthCheckStatus = iota
	// HealthCheckContainerStopped means the health check cannot
	// be run because the container is stopped
	HealthCheckContainerStopped HealthCheckStatus = iota
	// HealthCheckContainerNotFound means the container could
	// not be found in local store
	HealthCheckContainerNotFound HealthCheckStatus = iota
	// HealthCheckNotDefined means the container has no health
	// check defined in it
	HealthCheckNotDefined HealthCheckStatus = iota
	// HealthCheckInternalError means some something failed obtaining or running
	// a given health check
	HealthCheckInternalError HealthCheckStatus = iota
	// HealthCheckDefined means the healthcheck was found on the container
	HealthCheckDefined HealthCheckStatus = iota
)

// Healthcheck defaults.  These are used both in the cli as well in
// libpod and were moved from cmd/podman/common
const (
	// DefaultHealthCheckInterval default value
	DefaultHealthCheckInterval = "30s"
	// DefaultHealthCheckRetries default value
	DefaultHealthCheckRetries uint = 3
	// DefaultHealthCheckStartPeriod default value
	DefaultHealthCheckStartPeriod = "0s"
	// DefaultHealthCheckTimeout default value
	DefaultHealthCheckTimeout = "30s"
)

// HealthConfig.Test options
const (
	// HealthConfigTestNone disables healthcheck
	HealthConfigTestNone = "NONE"
	// HealthConfigTestCmd execs arguments directly
	HealthConfigTestCmd = "CMD"
	// HealthConfigTestCmdShell runs commands with the system's default shell
	HealthConfigTestCmdShell = "CMD-SHELL"
)

// HealthCheckOnFailureAction defines how Podman reacts when a container's health
// status turns unhealthy.
type HealthCheckOnFailureAction int

// Healthcheck on-failure actions.
const (
	// HealthCheckOnFailureActionNonce instructs Podman to not react on an unhealthy status.
	HealthCheckOnFailureActionNone = iota // Must be first iota for backwards compatibility
	// HealthCheckOnFailureActionInvalid denotes an invalid on-failure policy.
	HealthCheckOnFailureActionInvalid = iota
	// HealthCheckOnFailureActionNonce instructs Podman to kill the container on an unhealthy status.
	HealthCheckOnFailureActionKill = iota
	// HealthCheckOnFailureActionNonce instructs Podman to restart the container on an unhealthy status.
	HealthCheckOnFailureActionRestart = iota
	// HealthCheckOnFailureActionNonce instructs Podman to stop the container on an unhealthy status.
	HealthCheckOnFailureActionStop = iota
)

// String representations for on-failure actions.
const (
	strHealthCheckOnFailureActionNone    = "none"
	strHealthCheckOnFailureActionInvalid = "invalid"
	strHealthCheckOnFailureActionKill    = "kill"
	strHealthCheckOnFailureActionRestart = "restart"
	strHealthCheckOnFailureActionStop    = "stop"
)

// SupportedHealthCheckOnFailureActions lists all supported healthcheck restart policies.
var SupportedHealthCheckOnFailureActions = []string{
	strHealthCheckOnFailureActionNone,
	strHealthCheckOnFailureActionKill,
	strHealthCheckOnFailureActionRestart,
	strHealthCheckOnFailureActionStop,
}

// String returns the string representation of the HealthCheckOnFailureAction.
func (h HealthCheckOnFailureAction) String() string {
	switch h {
	case HealthCheckOnFailureActionNone:
		return strHealthCheckOnFailureActionNone
	case HealthCheckOnFailureActionKill:
		return strHealthCheckOnFailureActionKill
	case HealthCheckOnFailureActionRestart:
		return strHealthCheckOnFailureActionRestart
	case HealthCheckOnFailureActionStop:
		return strHealthCheckOnFailureActionStop
	default:
		return strHealthCheckOnFailureActionInvalid
	}
}

// ParseHealthCheckOnFailureAction parses the specified string into a HealthCheckOnFailureAction.
// An error is returned for an invalid input.
func ParseHealthCheckOnFailureAction(s string) (HealthCheckOnFailureAction, error) {
	switch s {
	case "", strHealthCheckOnFailureActionNone:
		return HealthCheckOnFailureActionNone, nil
	case strHealthCheckOnFailureActionKill:
		return HealthCheckOnFailureActionKill, nil
	case strHealthCheckOnFailureActionRestart:
		return HealthCheckOnFailureActionRestart, nil
	case strHealthCheckOnFailureActionStop:
		return HealthCheckOnFailureActionStop, nil
	default:
		err := fmt.Errorf("invalid on-failure action %q for health check: supported actions are %s", s, strings.Join(SupportedHealthCheckOnFailureActions, ","))
		return HealthCheckOnFailureActionInvalid, err
	}
}
