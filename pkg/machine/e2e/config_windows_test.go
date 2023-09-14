package e2e_test

import (
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/wsl"
	. "github.com/onsi/ginkgo/v2"
)

const podmanBinary = "../../../bin/windows/podman.exe"

func getDownloadLocation(_ machine.VirtProvider) string {
	fd, err := wsl.NewFedoraDownloader(machine.WSLVirt, "", defaultStream.String())
	if err != nil {
		Fail("unable to get WSL virtual image")
	}
	return fd.Get().URL.String()
}
