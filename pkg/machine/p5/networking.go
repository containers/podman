package p5

import (
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/containers/common/pkg/config"
	gvproxy "github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/define"
	"github.com/containers/podman/v4/pkg/machine/vmconfigs"
	"github.com/sirupsen/logrus"
)

const (
	dockerSock           = "/var/run/docker.sock"
	defaultGuestSock     = "/run/user/%d/podman/podman.sock"
	dockerConnectTimeout = 5 * time.Second
)

func startNetworking(mc *vmconfigs.MachineConfig, provider vmconfigs.VMStubber) (string, machine.APIForwardingState, error) {
	var (
		forwardingState machine.APIForwardingState
		forwardSock     string
	)
	// the guestSock is "inside" the guest machine
	guestSock := fmt.Sprintf(defaultGuestSock, mc.HostUser.UID)
	forwardUser := mc.SSH.RemoteUsername

	// TODO should this go up the stack higher
	if mc.HostUser.Rootful {
		guestSock = "/run/podman/podman.sock"
		forwardUser = "root"
	}

	cfg, err := config.Default()
	if err != nil {
		return "", 0, err
	}

	binary, err := cfg.FindHelperBinary(machine.ForwarderBinaryName, false)
	if err != nil {
		return "", 0, err
	}

	dataDir, err := mc.DataDir()
	if err != nil {
		return "", 0, err
	}
	hostSocket, err := dataDir.AppendToNewVMFile("podman.sock", nil)
	if err != nil {
		return "", 0, err
	}

	runDir, err := mc.RuntimeDir()
	if err != nil {
		return "", 0, err
	}

	linkSocketPath := filepath.Dir(dataDir.GetPath())
	linkSocket, err := define.NewMachineFile(filepath.Join(linkSocketPath, "podman.sock"), nil)
	if err != nil {
		return "", 0, err
	}

	cmd := gvproxy.NewGvproxyCommand()

	// GvProxy PID file path is now derived
	cmd.PidFile = filepath.Join(runDir.GetPath(), "gvproxy.pid")

	// TODO This can be re-enabled when gvisor-tap-vsock #305 is merged
	// debug is set, we dump to a logfile as well
	// if logrus.IsLevelEnabled(logrus.DebugLevel) {
	// 	cmd.LogFile = filepath.Join(runDir.GetPath(), "gvproxy.log")
	// }

	cmd.SSHPort = mc.SSH.Port

	cmd.AddForwardSock(hostSocket.GetPath())
	cmd.AddForwardDest(guestSock)
	cmd.AddForwardUser(forwardUser)
	cmd.AddForwardIdentity(mc.SSH.IdentityPath)

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		cmd.Debug = true
		logrus.Debug(cmd)
	}

	// This allows a provider to perform additional setup as well as
	// add in any provider specific options for gvproxy
	if err := provider.StartNetworking(mc, &cmd); err != nil {
		return "", 0, err
	}

	if mc.HostUser.UID != -1 {
		forwardSock, forwardingState = setupAPIForwarding(hostSocket, linkSocket)
	}

	c := cmd.Cmd(binary)
	if err := c.Start(); err != nil {
		return forwardSock, 0, fmt.Errorf("unable to execute: %q: %w", cmd.ToCmdline(), err)
	}

	return forwardSock, forwardingState, nil
}

type apiOptions struct { //nolint:unused
	socketpath, destinationSocketPath *define.VMFile
	fowardUser                        string
}

func setupAPIForwarding(hostSocket, linkSocket *define.VMFile) (string, machine.APIForwardingState) {
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

// conductVMReadinessCheck checks to make sure the machine is in the proper state
// and that SSH is up and running
func conductVMReadinessCheck(mc *vmconfigs.MachineConfig, maxBackoffs int, backoff time.Duration, stateF func() (define.Status, error)) (connected bool, sshError error, err error) {
	for i := 0; i < maxBackoffs; i++ {
		if i > 0 {
			time.Sleep(backoff)
			backoff *= 2
		}
		state, err := stateF()
		if err != nil {
			return false, nil, err
		}
		if state == define.Running && isListening(mc.SSH.Port) {
			// Also make sure that SSH is up and running.  The
			// ready service's dependencies don't fully make sure
			// that clients can SSH into the machine immediately
			// after boot.
			//
			// CoreOS users have reported the same observation but
			// the underlying source of the issue remains unknown.

			if sshError = machine.CommonSSH(mc.SSH.RemoteUsername, mc.SSH.IdentityPath, mc.Name, mc.SSH.Port, []string{"true"}); sshError != nil {
				logrus.Debugf("SSH readiness check for machine failed: %v", sshError)
				continue
			}
			connected = true
			break
		}
	}
	return
}

func isListening(port int) bool {
	// Check if we can dial it
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", "127.0.0.1", port), 10*time.Millisecond)
	if err != nil {
		return false
	}
	if err := conn.Close(); err != nil {
		logrus.Error(err)
	}
	return true
}
