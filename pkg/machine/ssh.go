package machine

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// LocalhostSSH is a common function for ssh'ing to a podman machine using system-connections
// and a port
// TODO This should probably be taught about an machineconfig to reduce input
func LocalhostSSH(username, identityPath, name string, sshPort int, inputArgs []string) error {
	return localhostBuiltinSSH(username, identityPath, name, sshPort, inputArgs, true, os.Stdin)
}

func LocalhostSSHShell(username, identityPath, name string, sshPort int, inputArgs []string) error {
	return localhostNativeSSH(username, identityPath, name, sshPort, inputArgs, os.Stdin)
}

func LocalhostSSHSilent(username, identityPath, name string, sshPort int, inputArgs []string) error {
	return localhostBuiltinSSH(username, identityPath, name, sshPort, inputArgs, false, nil)
}

func LocalhostSSHWithStdin(username, identityPath, name string, sshPort int, inputArgs []string, stdin io.Reader) error {
	return localhostBuiltinSSH(username, identityPath, name, sshPort, inputArgs, true, stdin)
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

func localhostNativeSSH(username, identityPath, name string, sshPort int, inputArgs []string, stdin io.Reader) error {
	sshDestination := username + "@localhost"
	port := strconv.Itoa(sshPort)
	interactive := true

	args := append([]string{"-i", identityPath, "-p", port, sshDestination}, LocalhostSSHArgs()...) // WARNING: This MUST NOT be generalized to allow communication over untrusted networks.
	if len(inputArgs) > 0 {
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
		"-o", "SetEnv=LC_ALL="}
}
