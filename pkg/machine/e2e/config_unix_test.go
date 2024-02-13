//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package e2e_test

import (
	"os/exec"
)

func pgrep(n string) (string, error) {
	out, err := exec.Command("pgrep", "gvproxy").Output()
	return string(out), err
}
