//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package e2e_test

import (
	"os/exec"
)

// TODO temporarily suspended
// func getDownloadLocation(p machine.VirtProvider) string {
// 	return getFCOSDownloadLocation(p)
// }

func pgrep(n string) (string, error) {
	out, err := exec.Command("pgrep", "gvproxy").Output()
	return string(out), err
}
