// +build linux

package cni_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/containers/podman/v3/libpod/network/cni"
	"github.com/containers/podman/v3/libpod/network/types"
	"github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var cniPluginDirs = []string{
	"/usr/libexec/cni",
	"/usr/lib/cni",
	"/usr/local/lib/cni",
	"/opt/cni/bin",
}

func TestCni(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CNI Suite")
}

func getNetworkInterface(cniConfDir string, machine bool) (types.ContainerNetwork, error) {
	return cni.NewCNINetworkInterface(cni.InitConfig{
		CNIConfigDir:  cniConfDir,
		CNIPluginDirs: cniPluginDirs,
		IsMachine:     machine,
		LockFile:      filepath.Join(cniConfDir, "cni.lock"),
	})
}

func SkipIfNoDnsname() {
	for _, path := range cniPluginDirs {
		f, err := os.Stat(filepath.Join(path, "dnsname"))
		if err == nil && f.Mode().IsRegular() {
			return
		}
	}
	Skip("dnsname cni plugin needs to be installed for this test")
}

func SkipIfNotFedora(msg string) {
	info := utils.GetHostDistributionInfo()
	if info.Distribution != "fedora" {
		Skip("Test can only run on Fedora: " + msg)
	}
}
