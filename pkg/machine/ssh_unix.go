//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package machine

import (
	"os"
	"os/exec"
)

func setupIOPassthrough(cmd *exec.Cmd, interactive bool) error {
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return nil
}
