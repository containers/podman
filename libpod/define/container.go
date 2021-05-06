package define

// Valid restart policy types.
const (
	// RestartPolicyNone indicates that no restart policy has been requested
	// by a container.
	RestartPolicyNone = ""
	// RestartPolicyNo is identical in function to RestartPolicyNone.
	RestartPolicyNo = "no"
	// RestartPolicyAlways unconditionally restarts the container.
	RestartPolicyAlways = "always"
	// RestartPolicyOnFailure restarts the container on non-0 exit code,
	// with an optional maximum number of retries.
	RestartPolicyOnFailure = "on-failure"
	// RestartPolicyUnlessStopped unconditionally restarts unless stopped
	// by the user. It is identical to Always except with respect to
	// handling of system restart, which Podman does not yet support.
	RestartPolicyUnlessStopped = "unless-stopped"
)

// RestartPolicyMap maps between restart-policy valid values to restart policy types
var RestartPolicyMap = map[string]string{
	"none":                     RestartPolicyNone,
	RestartPolicyNo:            RestartPolicyNo,
	RestartPolicyAlways:        RestartPolicyAlways,
	RestartPolicyOnFailure:     RestartPolicyOnFailure,
	RestartPolicyUnlessStopped: RestartPolicyUnlessStopped,
}
