//go:build !remote && (linux || freebsd) && !systemd

package libpod

import "github.com/sirupsen/logrus"

// addHealthCheckArgs adds healthcheck-related arguments to conmon for non-systemd builds
func (r *ConmonOCIRuntime) addHealthCheckArgs(ctr *Container, args []string) []string {
	// Add healthcheck flag if container has healthcheck config (non-systemd builds only)
	// For systemd builds, healthchecks are managed by systemd, not conmon
	if ctr.HasHealthCheck() {
		logrus.Debugf("HEALTHCHECK: Adding --enable-healthcheck flag for container %s", ctr.ID())
		args = append(args, "--enable-healthcheck")
	} else {
		logrus.Debugf("HEALTHCHECK: Container %s does not have healthcheck config, skipping --enable-healthcheck flag", ctr.ID())
	}
	return args
}
