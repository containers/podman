package define

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
