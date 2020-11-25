// +build !windows

package seccomp

import (
	"bufio"
	"errors"
	"os"
	"strings"

	perrors "github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

const statusFilePath = "/proc/self/status"

// IsSupported returns true if the system has been configured to support
// seccomp.
func IsSupported() bool {
	// Since Linux 3.8, the Seccomp field of the /proc/[pid]/status file
	// provides a method of obtaining the same information, without the risk
	// that the process is killed; see proc(5).
	status, err := parseStatusFile(statusFilePath)
	if err == nil {
		_, ok := status["Seccomp"]
		return ok
	}

	// PR_GET_SECCOMP (since Linux 2.6.23)
	// Return (as the function result) the secure computing mode of the calling
	// thread. If the caller is not in secure computing mode, this operation
	// returns 0; if the caller is in strict secure computing mode, then the
	// prctl() call will cause a SIGKILL signal to be sent to the process. If
	// the caller is in filter mode, and this system call is allowed by the
	// seccomp filters, it returns 2; otherwise, the process is killed with a
	// SIGKILL signal. This operation is available only if the kernel is
	// configured with CONFIG_SECCOMP enabled.
	if err := unix.Prctl(unix.PR_GET_SECCOMP, 0, 0, 0, 0); !errors.Is(err, unix.EINVAL) {
		// Make sure the kernel has CONFIG_SECCOMP_FILTER.
		if err := unix.Prctl(unix.PR_SET_SECCOMP, unix.SECCOMP_MODE_FILTER, 0, 0, 0); !errors.Is(err, unix.EINVAL) {
			return true
		}
	}

	return false
}

// parseStatusFile reads the provided `file` into a map of strings.
func parseStatusFile(file string) (map[string]string, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, perrors.Wrapf(err, "open status file %s", file)
	}
	defer f.Close()

	status := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		text := scanner.Text()
		parts := strings.SplitN(text, ":", 2)

		if len(parts) <= 1 {
			continue
		}

		status[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}

	if err := scanner.Err(); err != nil {
		return nil, perrors.Wrapf(err, "scan status file %s", file)
	}

	return status, nil
}
