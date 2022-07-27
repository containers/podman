//go:build amd64 || arm64
// +build amd64 arm64

package machine

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
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
	if err := os.MkdirAll(filepath.Dir(writeLocation), 0700); err != nil {
		return "", err
	}
	if err := generatekeys(writeLocation); err != nil {
		return "", err
	}
	b, err := ioutil.ReadFile(writeLocation + ".pub")
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(b), "\n"), nil
}

func CreateSSHKeysPrefix(dir string, file string, passThru bool, skipExisting bool, prefix ...string) (string, error) {
	location := filepath.Join(dir, file)

	_, e := os.Stat(location)
	if !skipExisting || errors.Is(e, os.ErrNotExist) {
		if err := generatekeysPrefix(dir, file, passThru, prefix...); err != nil {
			return "", err
		}
	} else {
		fmt.Println("Keys already exist, reusing")
	}
	b, err := ioutil.ReadFile(filepath.Join(dir, file) + ".pub")
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
func generatekeysPrefix(dir string, file string, passThru bool, prefix ...string) error {
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
