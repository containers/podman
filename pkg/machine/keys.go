// +build amd64 arm64

package machine

import (
	"errors"
	"fmt"
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
	return exec.Command(sshCommand[0], args...).Run()
}

// generatekeys creates an ed25519 set of keys
func generatekeysPrefix(dir string, file string, passThru bool, prefix ...string) error {
	args := append([]string{}, prefix[1:]...)
	args = append(args, sshCommand...)
	args = append(args, file)
	cmd := exec.Command(prefix[0], args...)
	cmd.Dir = dir
	if passThru {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	logrus.Debugf("Running wsl cmd %v in dir: %s", args, dir)
	return cmd.Run()
}
