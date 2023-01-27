package machine

import (
	"errors"
	"os"
	"os/exec"
	"strings"
)

func getLocalTimeZone() (string, error) {
	output, err := exec.Command("timedatectl", "show", "--property=Timezone").Output()
	if errors.Is(err, exec.ErrNotFound) {
		output, err = os.ReadFile("/etc/timezone")
	}
	if err != nil {
		return "", err
	}
	// Remove prepended field and the newline
	return strings.TrimPrefix(strings.TrimSuffix(string(output), "\n"), "Timezone="), nil
}
