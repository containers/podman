// +build linux

package netavark_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/containers/podman/v3/libpod/network/netavark"
	"github.com/containers/podman/v3/libpod/network/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestNetavark(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Netavark Suite")
}

var netavarkBinary string

func init() {
	netavarkBinary = os.Getenv("NETAVARK_BINARY")
	if netavarkBinary == "" {
		netavarkBinary = "/usr/libexec/podman/netavark"
	}
}

func getNetworkInterface(confDir string, machine bool) (types.ContainerNetwork, error) {
	return netavark.NewNetworkInterface(netavark.InitConfig{
		NetworkConfigDir: confDir,
		IsMachine:        machine,
		NetavarkBinary:   netavarkBinary,
		LockFile:         filepath.Join(confDir, "netavark.lock"),
	})
}
