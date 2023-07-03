package machine

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"

	"github.com/sirupsen/logrus"
)

// CommonSSH is a common function for ssh'ing to a podman machine using system-connections
// and a port
func CommonSSH(username, identityPath, name string, sshPort int, inputArgs []string) error {
	sshDestination := username + "@localhost"
	port := strconv.Itoa(sshPort)

	args := []string{"-i", identityPath, "-p", port, sshDestination,
		"-o", "IdentitiesOnly=yes",
		"-o", "StrictHostKeyChecking=no", "-o", "LogLevel=ERROR", "-o", "SetEnv=LC_ALL="}
	if len(inputArgs) > 0 {
		args = append(args, inputArgs...)
	} else {
		fmt.Printf("Connecting to vm %s. To close connection, use `~.` or `exit`\n", name)
	}

	cmd := exec.Command("ssh", args...)
	logrus.Debugf("Executing: ssh %v\n", args)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("SSH failed: %w (stdout: %q, stderr: %q)", err, stdout.String(), stderr.String())
	}

	return nil
}
