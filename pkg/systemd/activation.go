package systemd

import (
	"os"
	"strconv"
	"strings"
)

// SocketActivated determine if podman is running under the socket activation protocol
func SocketActivated() bool {
	pid, pid_found := os.LookupEnv("LISTEN_PID")
	fds, fds_found := os.LookupEnv("LISTEN_FDS")
	fdnames, fdnames_found := os.LookupEnv("LISTEN_FDNAMES")

	if !(pid_found && fds_found && fdnames_found) {
		return false
	}

	p, err := strconv.Atoi(pid)
	if err != nil || p != os.Getpid() {
		return false
	}

	nfds, err := strconv.Atoi(fds)
	if err != nil || nfds < 1 {
		return false
	}

	// First available file descriptor is always 3.
	if nfds > 1 {
		names := strings.Split(fdnames, ":")
		for _, n := range names {
			if strings.Contains(n, "podman") {
				return true
			}
		}
	}

	return true
}
