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

// ParseUserNamespace returns the content of /proc/$pid/ns/user.
func ParseUserNamespace(pid string) (string, error) {
	userNS, err := os.Readlink(fmt.Sprintf("/proc/%s/ns/user", pid))
	if err != nil {
		return "", err
	}
	return userNS, nil
}
