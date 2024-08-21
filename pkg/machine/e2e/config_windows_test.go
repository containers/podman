package e2e_test

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/containers/podman/v5/pkg/machine/define"
)

const podmanBinary = "../../../bin/windows/podman.exe"

// pgrep emulates the pgrep linux command
func pgrep(n string) (string, error) {
	// add filter to find the process and do no display a header
	args := []string{"/fi", fmt.Sprintf("IMAGENAME eq %s", n), "/nh"}
	out, err := exec.Command("tasklist.exe", args...).Output()
	if err != nil {
		return "", err
	}
	strOut := string(out)
	// in pgrep, if no running process is found, it exits 1 and the output is zilch
	if strings.Contains(strOut, "INFO: No tasks are running which match the specified search") {
		return "", fmt.Errorf("no task found")
	}
	return strOut, nil
}

func getOtherProvider() string {
	if isVmtype(define.WSLVirt) {
		return "hyperv"
	} else if isVmtype(define.HyperVVirt) {
		return "wsl"
	}
	return ""
}
