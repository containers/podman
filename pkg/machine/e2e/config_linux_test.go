package e2e_test

import "os/exec"

const podmanBinary = "../../../bin/podman-remote"

func pgrep(n string) (string, error) {
	out, err := exec.Command("pgrep", "gvproxy").Output()
	return string(out), err
}
