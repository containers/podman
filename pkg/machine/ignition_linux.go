package machine

import (
	"os/exec"
	"strings"
)

func getLocalTimeZone() (string, error) {
	output, err := exec.Command("timedatectl", "show", "--property=Timezone").Output()
	if err != nil {
		return "", err
	}
	// Remove prepended field and the newline
	return strings.TrimPrefix(strings.TrimSuffix(string(output), "\n"), "Timezone="), nil
}
