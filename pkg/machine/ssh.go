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

// CommonSSH is a common function for ssh'ing to a podman machine using system-connections
// and a port
// TODO This should probably be taught about an machineconfig to reduce input
func CommonSSH(username, identityPath, name string, sshPort int, inputArgs []string) error {
	return commonBuiltinSSH(username, identityPath, name, sshPort, inputArgs, true, os.Stdin)
}

func CommonSSHShell(username, identityPath, name string, sshPort int, inputArgs []string) error {
	return commonNativeSSH(username, identityPath, name, sshPort, inputArgs, os.Stdin)
}

func CommonSSHSilent(username, identityPath, name string, sshPort int, inputArgs []string) error {
	return commonBuiltinSSH(username, identityPath, name, sshPort, inputArgs, false, nil)
}

func CommonSSHWithStdin(username, identityPath, name string, sshPort int, inputArgs []string, stdin io.Reader) error {
	return commonBuiltinSSH(username, identityPath, name, sshPort, inputArgs, true, stdin)
}

func commonBuiltinSSH(username, identityPath, name string, sshPort int, inputArgs []string, passOutput bool, stdin io.Reader) error {
	config, err := createConfig(username, identityPath)
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

func createConfig(user string, identityPath string) (*ssh.ClientConfig, error) {
	key, err := os.ReadFile(identityPath)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}

	return &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}, nil
}

func commonNativeSSH(username, identityPath, name string, sshPort int, inputArgs []string, stdin io.Reader) error {
	sshDestination := username + "@localhost"
	port := strconv.Itoa(sshPort)
	interactive := true

	args := []string{"-i", identityPath, "-p", port, sshDestination,
		"-o", "IdentitiesOnly=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=" + os.DevNull,
		"-o", "CheckHostIP=no",
		"-o", "LogLevel=ERROR", "-o", "SetEnv=LC_ALL="}
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
