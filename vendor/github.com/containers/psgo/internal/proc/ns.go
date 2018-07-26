package proc

import (
	"fmt"
	"os"
)

// ParsePIDNamespace returns the content of /proc/$pid/ns/pid.
func ParsePIDNamespace(pid string) (string, error) {
	pidNS, err := os.Readlink(fmt.Sprintf("/proc/%s/ns/pid", pid))
	if err != nil {
		return "", err
	}
	return pidNS, nil
}
