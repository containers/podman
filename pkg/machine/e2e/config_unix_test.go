//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package e2e_test

import (
	"os/exec"

	"github.com/containers/podman/v4/pkg/machine"
	. "github.com/onsi/ginkgo/v2"
)

func getDownloadLocation(p machine.VirtProvider) string {
	dd, err := p.NewDownload("")
	if err != nil {
		Fail("unable to create new download")
	}

	fcd, err := dd.GetFCOSDownload(defaultStream)
	if err != nil {
		Fail("unable to get virtual machine image")
	}

	return fcd.Location
}

func pgrep(n string) (string, error) {
	out, err := exec.Command("pgrep", "gvproxy").Output()
	return string(out), err
}
