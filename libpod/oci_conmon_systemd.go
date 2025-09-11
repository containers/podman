//go:build !remote && (linux || freebsd) && systemd

package libpod

// addHealthCheckArgs adds healthcheck-related arguments to conmon for systemd builds
func (r *ConmonOCIRuntime) addHealthCheckArgs(ctr *Container, args []string) []string {
	// For systemd builds, healthchecks are managed by systemd, not conmon
	// No healthcheck flags needed for conmon
	return args
}
