package systemdgen

import (
	"fmt"

	"github.com/pkg/errors"
)

var template = `[Unit]
Description=%s Podman Container
[Service]
Restart=%s
ExecStart=/usr/bin/podman start %s
ExecStop=/usr/bin/podman stop -t %d %s
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
	if err := ValidateRestartPolicy(restart); err != nil {
		return "", err
	}

	unit := fmt.Sprintf(template, name, restart, name, stopTimeout, name, pidFile)
	return unit, nil
}
