//go:build !windows
// +build !windows

package os

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"github.com/crc-org/crc/v2/pkg/crc/logging"
)

func WriteToFileAsRoot(reason, content, filepath string, mode os.FileMode) error {
	logging.Infof("Using root access: %s", reason)
	cmd := exec.Command("sudo", "tee", filepath) // #nosec G204
	cmd.Stdin = strings.NewReader(content)
	buf := new(bytes.Buffer)
	cmd.Stderr = buf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Failed writing to file as root: %s: %s: %v", filepath, buf.String(), err)
	}
	if _, _, err := RunPrivileged(fmt.Sprintf("Changing permissions for %s to %o ", filepath, mode),
		"chmod", strconv.FormatUint(uint64(mode), 8), filepath); err != nil {
		return err
	}
	return nil
}

func RemoveFileAsRoot(reason, filepath string) error {
	if !FileExists(filepath) {
		return nil
	}
	_, _, err := RunPrivileged(reason, "rm", "-fr", filepath)
	return err
}

func GetCurrentUsername() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	return u.Username, nil
}
