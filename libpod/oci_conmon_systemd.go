//go:build !remote && (linux || freebsd) && systemd

package libpod

// addHealthCheckArgs adds healthcheck-related arguments to conmon for systemd builds
func (r *ConmonOCIRuntime) addHealthCheckArgs(ctr *Container, args []string) []string {
	// For systemd builds, healthchecks are managed by systemd timers, not conmon
	// No healthcheck CLI arguments needed for conmon
	return args
}
