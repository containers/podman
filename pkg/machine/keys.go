//go:build amd64 || arm64
// +build amd64 arm64

package machine

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

var sshCommand = []string{"ssh-keygen", "-N", "", "-t", "ed25519", "-f"}

// CreateSSHKeys makes a priv and pub ssh key for interacting
// the a VM.
func CreateSSHKeys(writeLocation string) (string, error) {
	// If the SSH key already exists, hard fail
	if _, err := os.Stat(writeLocation); err == nil {
		return "", fmt.Errorf("SSH key already exists: %s", writeLocation)
	}
	if err := os.MkdirAll(filepath.Dir(writeLocation), 0700); err != nil {
		return "", err
	}
	if err := generatekeys(writeLocation); err != nil {
		return "", err
	}
	b, err := os.ReadFile(writeLocation + ".pub")
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(b), "\n"), nil
}

func CreateSSHKeysPrefix(identityPath string, passThru bool, skipExisting bool, prefix ...string) (string, error) {
	_, e := os.Stat(identityPath)
	if !skipExisting || errors.Is(e, os.ErrNotExist) {
		if err := generatekeysPrefix(identityPath, passThru, prefix...); err != nil {
			return "oh hai", err
		}
	} else {
		fmt.Println("Keys already exist, reusing")
	}
	b, err := os.ReadFile(identityPath + ".pub")
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(b), "\n"), nil
}

// generatekeys creates an ed25519 set of keys
func generatekeys(writeLocation string) error {
	args := append(append([]string{}, sshCommand[1:]...), writeLocation)
	cmd := exec.Command(sshCommand[0], args...)
	stdErr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	waitErr := cmd.Wait()
	if waitErr == nil {
		return nil
	}
	errMsg, err := io.ReadAll(stdErr)
	if err != nil {
		return fmt.Errorf("key generation failed, unable to read from stderr: %w", waitErr)
	}
	return fmt.Errorf("failed to generate keys: %s: %w", string(errMsg), waitErr)
}

// generatekeys creates an ed25519 set of keys
func generatekeysPrefix(identityPath string, passThru bool, prefix ...string) error {
	dir := filepath.Dir(identityPath)
	file := filepath.Base(identityPath)

	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("could not create ssh directory: %w", err)
	}

	args := append([]string{}, prefix[1:]...)
	args = append(args, sshCommand...)
	args = append(args, file)

	binary, err := exec.LookPath(prefix[0])
	if err != nil {
		return err
	}
	binary, err = filepath.Abs(binary)
	if err != nil {
		return err
	}
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	if passThru {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	logrus.Debugf("Running wsl cmd %v in dir: %s", args, dir)
	return cmd.Run()
}
