//go:build dragonfly || freebsd || linux || netbsd || openbsd || darwin

package shim

import (
	"io/fs"
	"net"
	"os"
	"path/filepath"

	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/sirupsen/logrus"
)

func setupMachineSockets(name string, dirs *define.MachineDirs) ([]string, string, machine.APIForwardingState, error) {
	hostSocket, err := dirs.DataDir.AppendToNewVMFile("podman.sock", nil)
	if err != nil {
		return nil, "", 0, err
	}

	linkSocketPath := filepath.Dir(dirs.DataDir.GetPath())
	linkSocket, err := define.NewMachineFile(filepath.Join(linkSocketPath, "podman.sock"), nil)
	if err != nil {
		return nil, "", 0, err
	}

	forwardSock, state := setupForwardingLinks(hostSocket, linkSocket)
	return []string{hostSocket.GetPath()}, forwardSock, state, nil
}

func setupForwardingLinks(hostSocket, linkSocket *define.VMFile) (string, machine.APIForwardingState) {
	// The linking pattern is /var/run/docker.sock -> user global sock (link) -> machine sock (socket)
	// This allows the helper to only have to maintain one constant target to the user, which can be
	// repositioned without updating docker.sock.

	if !dockerClaimSupported() {
		return hostSocket.GetPath(), machine.ClaimUnsupported
	}

	if !dockerClaimHelperInstalled() {
		return hostSocket.GetPath(), machine.NotInstalled
	}

	if !alreadyLinked(hostSocket.GetPath(), linkSocket.GetPath()) {
		if checkSockInUse(linkSocket.GetPath()) {
			return hostSocket.GetPath(), machine.MachineLocal
		}

		_ = linkSocket.Delete()

		if err := os.Symlink(hostSocket.GetPath(), linkSocket.GetPath()); err != nil {
			logrus.Warnf("could not create user global API forwarding link: %s", err.Error())
			return hostSocket.GetPath(), machine.MachineLocal
		}
	}

	if !alreadyLinked(linkSocket.GetPath(), dockerSock) {
		if checkSockInUse(dockerSock) {
			return hostSocket.GetPath(), machine.MachineLocal
		}

		if !claimDockerSock() {
			logrus.Warn("podman helper is installed, but was not able to claim the global docker sock")
			return hostSocket.GetPath(), machine.MachineLocal
		}
	}

	return dockerSock, machine.DockerGlobal
}

func alreadyLinked(target string, link string) bool {
	read, err := os.Readlink(link)
	return err == nil && read == target
}

func checkSockInUse(sock string) bool {
	if info, err := os.Stat(sock); err == nil && info.Mode()&fs.ModeSocket == fs.ModeSocket {
		_, err = net.DialTimeout("unix", dockerSock, dockerConnectTimeout)
		return err == nil
	}

	return false
}
