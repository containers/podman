package machine

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// LocalhostSSH is a common function for ssh'ing to a podman machine using system-connections
// and a port
// TODO This should probably be taught about an machineconfig to reduce input
func LocalhostSSH(username, identityPath, name string, sshPort int, inputArgs []string) error {
	return localhostBuiltinSSH(username, identityPath, name, sshPort, inputArgs, true, os.Stdin)
}

// LocalhostSSHShellForceTerm runs the native ssh shell client and forces a terminal (-t)
func LocalhostSSHShellForceTerm(username, identityPath, name string, sshPort int, inputArgs []string) error {
	return localhostNativeSSH(username, identityPath, name, sshPort, inputArgs, os.Stdin, true)
}

func LocalhostSSHShell(username, identityPath, name string, sshPort int, inputArgs []string) error {
	return localhostNativeSSH(username, identityPath, name, sshPort, inputArgs, os.Stdin, false)
}

func LocalhostSSHSilent(username, identityPath, name string, sshPort int, inputArgs []string) error {
	return localhostBuiltinSSH(username, identityPath, name, sshPort, inputArgs, false, nil)
}

func LocalhostSSHWithStdin(username, identityPath, name string, sshPort int, inputArgs []string, stdin io.Reader) error {
	return localhostBuiltinSSH(username, identityPath, name, sshPort, inputArgs, true, stdin)
}

// LocalhostSSHCopy uses scp to copy files from/to a localhost machine using ssh.
func LocalhostSSHCopy(username, identityPath string, sshPort int, srcPath, destPath string, isSrcFromGuest, quiet bool) error {
	var src, dest string
	if isSrcFromGuest {
		src = username + "@localhost:" + srcPath
		dest = destPath
	} else {
		src = srcPath
		dest = username + "@localhost:" + destPath
	}
	args := append(
		LocalhostSSHArgs(), // Warning: This MUST NOT be generalized to allow communication over untrusted networks.
		"-r",
		"-i", identityPath,
		"-P", strconv.Itoa(sshPort),
		src, dest)
	cmd := exec.Command("scp", args...)
	if !quiet {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// SSHSession holds a reusable SSH connection to a guest VM, allowing multiple
// commands to be run over a single TCP connection and SSH handshake.
type SSHSession struct {
	client *ssh.Client
	name   string
}

// NewSSHSession dials an SSH connection to the guest VM and returns a session
// that can run multiple commands without re-dialing.
func NewSSHSession(username, identityPath, name string, sshPort int) (*SSHSession, error) {
	config, err := createLocalhostConfig(username, identityPath)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	client, err := ssh.Dial("tcp", fmt.Sprintf("localhost:%d", sshPort), config)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("SSH session to %q opened: dial=%v", name, time.Since(start))
	return &SSHSession{client: client, name: name}, nil
}

func (s *SSHSession) Close() error {
	return s.client.Close()
}

// Run executes a command on the guest VM over the existing SSH connection.
func (s *SSHSession) Run(inputArgs []string, passOutput bool, stdin io.Reader) error {
	start := time.Now()
	session, err := s.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	cmd := strings.Join(inputArgs, " ")
	logrus.Debugf("Running ssh command on machine %q: %s", s.name, cmd)
	session.Stdin = stdin
	if passOutput {
		session.Stdout = os.Stdout
		session.Stderr = os.Stderr
	} else if logrus.IsLevelEnabled(logrus.DebugLevel) {
		err = runSessionWithDebug(session, cmd)
		logrus.Debugf("SSH to %q: dial=0s run=%v total=%v cmd=%s", s.name, time.Since(start), time.Since(start), cmd)
		return err
	}

	err = session.Run(cmd)
	logrus.Debugf("SSH to %q: dial=0s run=%v total=%v cmd=%s", s.name, time.Since(start), time.Since(start), cmd)
	return err
}

func localhostBuiltinSSH(username, identityPath, name string, sshPort int, inputArgs []string, passOutput bool, stdin io.Reader) error {
	config, err := createLocalhostConfig(username, identityPath) // WARNING: This MUST NOT be generalized to allow communication over untrusted networks.
	if err != nil {
		return err
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("localhost:%d", sshPort), config)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	cmd := strings.Join(inputArgs, " ")
	logrus.Debugf("Running ssh command on machine %q: %s", name, cmd)
	session.Stdin = stdin
	if passOutput {
		session.Stdout = os.Stdout
		session.Stderr = os.Stderr
	} else if logrus.IsLevelEnabled(logrus.DebugLevel) {
		return runSessionWithDebug(session, cmd)
	}

	return session.Run(cmd)
}

func runSessionWithDebug(session *ssh.Session, cmd string) error {
	outPipe, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	errPipe, err := session.StderrPipe()
	if err != nil {
		return err
	}
	logOuput := func(pipe io.Reader, done chan struct{}) {
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			logrus.Debugf("ssh output: %s", scanner.Text())
		}
		done <- struct{}{}
	}
	if err := session.Start(cmd); err != nil {
		return err
	}
	completed := make(chan struct{}, 2)
	go logOuput(outPipe, completed)
	go logOuput(errPipe, completed)
	<-completed
	<-completed

	return session.Wait()
}

// createLocalhostConfig returns a *ssh.ClientConfig for authenticating a user using a private key
//
// WARNING: This MUST NOT be used to communicate over untrusted networks.
func createLocalhostConfig(user string, identityPath string) (*ssh.ClientConfig, error) {
	key, err := os.ReadFile(identityPath)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}

	return &ssh.ClientConfig{
		// Not specifying ciphers / MACs seems to allow fairly weak ciphers. This config is restricted
		// to connecting to localhost: where we rely on the kernel’s process isolation, not primarily on cryptography.
		User: user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		// This config is restricted to connecting to localhost (and to a VM we manage),
		// we rely on the kernel’s process isolation, not on cryptography,
		// This would be UNACCEPTABLE for most other uses.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}, nil
}

func localhostNativeSSH(username, identityPath, name string, sshPort int, inputArgs []string, stdin io.Reader, forceTerm bool) error {
	sshDestination := username + "@localhost"
	port := strconv.Itoa(sshPort)
	interactive := true

	args := append(LocalhostSSHArgs(), // WARNING: This MUST NOT be generalized to allow communication over untrusted networks.
		"-i", identityPath,
		"-p", port,
		sshDestination)
	if len(inputArgs) > 0 {
		// on the other condition, the term is forced
		// anyway
		if forceTerm {
			args = append(args, "-t")
		}
		interactive = false
		args = append(args, inputArgs...)
	} else {
		// ensure we have a tty
		args = append(args, "-t")
		fmt.Printf("Connecting to vm %s. To close connection, use `~.` or `exit`\n", name)
	}

	cmd := exec.Command("ssh", args...)
	logrus.Debugf("Executing: ssh %v\n", args)

	if err := setupIOPassthrough(cmd, interactive, stdin); err != nil {
		return err
	}

	return cmd.Run()
}

// LocalhostSSHArgs returns OpenSSH command-line options for connecting with no host key identity checks.
//
// WARNING: This MUST NOT be used to communicate over untrusted networks.
func LocalhostSSHArgs() []string {
	// This config is restricted to connecting to localhost (and to a VM we manage),
	// we rely on the kernel’s process isolation, not on cryptography,
	// This would be UNACCEPTABLE for most other uses.
	return []string{
		"-o", "IdentitiesOnly=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=" + os.DevNull,
		"-o", "CheckHostIP=no",
		"-o", "LogLevel=ERROR",
		"-o", "SetEnv=LC_ALL=",
	}
}
