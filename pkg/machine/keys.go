// +build amd64,linux arm64,linux amd64,darwin arm64,darwin

package machine

import (
	"io/ioutil"
	"os/exec"
	"strings"
)

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

// generatekeys creates an ed25519 set of keys
func generatekeys(writeLocation string) error {
	return exec.Command("ssh-keygen", "-N", "", "-t", "ed25519", "-f", writeLocation).Run()
}
