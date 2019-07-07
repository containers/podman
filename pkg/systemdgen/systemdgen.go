package systemdgen

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var template = `[Unit]
Description=%s Podman Container
[Service]
Restart=%s
ExecStart=%s start %s
ExecStop=%s stop -t %d %s
KillMode=none
Type=forking
PIDFile=%s
[Install]
WantedBy=multi-user.target`

var restartPolicies = []string{"no", "on-success", "on-failure", "on-abnormal", "on-watchdog", "on-abort", "always"}

// ValidateRestartPolicy checks that the user-provided policy is valid
func ValidateRestartPolicy(restart string) error {
	for _, i := range restartPolicies {
		if i == restart {
			return nil
		}
	}
	return errors.Errorf("%s is not a valid restart policy", restart)
}

// CreateSystemdUnitAsString takes variables to create a systemd unit file used to control
// a libpod container
func CreateSystemdUnitAsString(name, cid, restart, pidFile string, stopTimeout int) (string, error) {
	podmanExe := getPodmanExecutable()
	return createSystemdUnitAsString(podmanExe, name, cid, restart, pidFile, stopTimeout)
}

func createSystemdUnitAsString(exe, name, cid, restart, pidFile string, stopTimeout int) (string, error) {
	if err := ValidateRestartPolicy(restart); err != nil {
		return "", err
	}

	unit := fmt.Sprintf(template, name, restart, exe, name, exe, stopTimeout, name, pidFile)
	return unit, nil
}

func getPodmanExecutable() string {
	podmanExe, err := os.Executable()
	if err != nil {
		podmanExe = "/usr/bin/podman"
		logrus.Warnf("Could not obtain podman executable location, using default %s", podmanExe)
	}

	return podmanExe
}
