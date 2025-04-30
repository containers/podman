package ignition

import (
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

func getLocalTimeZone() (string, error) {
	trimTzFunc := func(s string) string {
		return strings.TrimPrefix(strings.TrimSuffix(s, "\n"), "Timezone=")
	}

	// perform a variety of ways to see if we can determine the tz
	output, err := exec.Command("timedatectl", "show", "--property=Timezone").Output()
	if err == nil {
		return trimTzFunc(string(output)), nil
	}
	logrus.Debugf("Timedatectl show --property=Timezone failed: %s", err)
	output, err = os.ReadFile("/etc/timezone")
	if err == nil {
		return trimTzFunc(string(output)), nil
	}
	logrus.Debugf("unable to read /etc/timezone, falling back to empty timezone: %s", err)
	// if we cannot determine the tz, return empty string
	return "", nil
}
