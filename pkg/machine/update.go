//go:build amd64 || arm64

package machine

import (
	"fmt"

	"github.com/containers/podman/v5/pkg/machine/ignition"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"github.com/sirupsen/logrus"
)

func UpdatePodmanDockerSockService(mc *vmconfigs.MachineConfig) error {
	content := ignition.GetPodmanDockerTmpConfig(mc.HostUser.UID, mc.HostUser.Rootful, false)
	command := fmt.Sprintf("'echo %q > %s'", content, ignition.PodmanDockerTmpConfPath)
	args := []string{"sudo", "bash", "-c", command}
	if err := CommonSSH(mc.SSH.RemoteUsername, mc.SSH.IdentityPath, mc.Name, mc.SSH.Port, args); err != nil {
		logrus.Warnf("Could not update internal docker sock config")
		return err
	}

	args = []string{"sudo", "systemd-tmpfiles", "--create", "--prefix=/run/docker.sock"}
	if err := CommonSSH(mc.SSH.RemoteUsername, mc.SSH.IdentityPath, mc.Name, mc.SSH.Port, args); err != nil {
		logrus.Warnf("Could not create internal docker sock")
		return err
	}

	return nil
}
