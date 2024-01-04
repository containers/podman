//go:build amd64 || arm64

package machine

import (
	"fmt"

	"github.com/containers/podman/v4/pkg/machine/ignition"
	"github.com/sirupsen/logrus"
)

func UpdatePodmanDockerSockService(vm VM, name string, uid int, rootful bool) error {
	content := ignition.GetPodmanDockerTmpConfig(uid, rootful, false)
	command := fmt.Sprintf("'echo %q > %s'", content, ignition.PodmanDockerTmpConfPath)
	args := []string{"sudo", "bash", "-c", command}
	if err := vm.SSH(name, SSHOptions{Args: args}); err != nil {
		logrus.Warnf("Could not not update internal docker sock config")
		return err
	}

	args = []string{"sudo", "systemd-tmpfiles", "--create", "--prefix=/run/docker.sock"}
	if err := vm.SSH(name, SSHOptions{Args: args}); err != nil {
		logrus.Warnf("Could not create internal docker sock")
		return err
	}

	return nil
}
