// +build linux

package netavark_test

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/containers/podman/v3/libpod/network/netavark"
	"github.com/containers/podman/v3/libpod/network/types"
	"github.com/containers/podman/v3/libpod/network/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomegaTypes "github.com/onsi/gomega/types"
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
		IPAMDBPath:       filepath.Join(confDir, "ipam.db"),
		LockFile:         filepath.Join(confDir, "netavark.lock"),
	})
}

// EqualSubnet is a custom GomegaMatcher to match a subnet
// This makes sure to not use the 16 bytes ip representation.
func EqualSubnet(subnet *net.IPNet) gomegaTypes.GomegaMatcher {
	return &equalSubnetMatcher{
		expected: subnet,
	}
}

type equalSubnetMatcher struct {
	expected *net.IPNet
}

func (m *equalSubnetMatcher) Match(actual interface{}) (bool, error) {
	util.NormalizeIP(&m.expected.IP)

	subnet, ok := actual.(*net.IPNet)
	if !ok {
		return false, fmt.Errorf("EqualSubnet expects a *net.IPNet")
	}
	util.NormalizeIP(&subnet.IP)

	return reflect.DeepEqual(subnet, m.expected), nil
}

func (m *equalSubnetMatcher) FailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected subnet %#v to equal subnet %#v", actual, m.expected)
}

func (m *equalSubnetMatcher) NegatedFailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected subnet %#v not to equal subnet %#v", actual, m.expected)
}
