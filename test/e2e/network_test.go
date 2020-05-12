// +build !remoteclient

package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	. "github.com/containers/libpod/test/utils"
	"github.com/containers/storage/pkg/stringid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func writeConf(conf []byte, confPath string) {
	if err := ioutil.WriteFile(confPath, conf, 777); err != nil {
		fmt.Println(err)
	}
}
func removeConf(confPath string) {
	if err := os.Remove(confPath); err != nil {
		fmt.Println(err)
	}
}

var _ = Describe("Podman network", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		SkipIfRootless()
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	var (
		secondConf = `{
    "cniVersion": "0.3.0",
    "name": "podman-integrationtest",
    "plugins": [
      {
        "type": "bridge",
        "bridge": "cni1",
        "isGateway": true,
        "ipMasq": true,
        "ipam": {
            "type": "host-local",
            "subnet": "10.99.0.0/16",
            "routes": [
                { "dst": "0.0.0.0/0" }
            ]
        }
      },
      {
        "type": "portmap",
        "capabilities": {
          "portMappings": true
        }
      }
    ]
}`
		cniPath = "/etc/cni/net.d"
	)

	It("podman network list", func() {
		// Setup, use uuid to prevent conflict with other tests
		uuid := stringid.GenerateNonCryptoID()
		secondPath := filepath.Join(cniPath, fmt.Sprintf("%s.conflist", uuid))
		writeConf([]byte(secondConf), secondPath)
		defer removeConf(secondPath)

		session := podmanTest.Podman([]string{"network", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputContains("podman-integrationtest")).To(BeTrue())
	})

	It("podman network list -q", func() {
		// Setup, use uuid to prevent conflict with other tests
		uuid := stringid.GenerateNonCryptoID()
		secondPath := filepath.Join(cniPath, fmt.Sprintf("%s.conflist", uuid))
		writeConf([]byte(secondConf), secondPath)
		defer removeConf(secondPath)

		session := podmanTest.Podman([]string{"network", "ls", "--quiet"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputContains("podman-integrationtest")).To(BeTrue())
	})

	It("podman network list --filter success", func() {
		// Setup, use uuid to prevent conflict with other tests
		uuid := stringid.GenerateNonCryptoID()
		secondPath := filepath.Join(cniPath, fmt.Sprintf("%s.conflist", uuid))
		writeConf([]byte(secondConf), secondPath)
		defer removeConf(secondPath)

		session := podmanTest.Podman([]string{"network", "ls", "--filter", "plugin=bridge"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputContains("podman-integrationtest")).To(BeTrue())
	})

	It("podman network list --filter failure", func() {
		// Setup, use uuid to prevent conflict with other tests
		uuid := stringid.GenerateNonCryptoID()
		secondPath := filepath.Join(cniPath, fmt.Sprintf("%s.conflist", uuid))
		writeConf([]byte(secondConf), secondPath)
		defer removeConf(secondPath)

		session := podmanTest.Podman([]string{"network", "ls", "--filter", "plugin=test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputContains("podman-integrationtest")).To(BeFalse())
	})

	It("podman network rm no args", func() {
		session := podmanTest.Podman([]string{"network", "rm"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).ToNot(BeZero())
	})

	It("podman network rm", func() {
		// Setup, use uuid to prevent conflict with other tests
		uuid := stringid.GenerateNonCryptoID()
		secondPath := filepath.Join(cniPath, fmt.Sprintf("%s.conflist", uuid))
		writeConf([]byte(secondConf), secondPath)
		defer removeConf(secondPath)

		session := podmanTest.Podman([]string{"network", "ls", "--quiet"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputContains("podman-integrationtest")).To(BeTrue())

		rm := podmanTest.Podman([]string{"network", "rm", "podman-integrationtest"})
		rm.WaitWithDefaultTimeout()
		Expect(rm.ExitCode()).To(BeZero())

		results := podmanTest.Podman([]string{"network", "ls", "--quiet"})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		Expect(results.LineInOutputContains("podman-integrationtest")).To(BeFalse())
	})

	It("podman network inspect no args", func() {
		session := podmanTest.Podman([]string{"network", "inspect"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).ToNot(BeZero())
	})

	It("podman network inspect", func() {
		// Setup, use uuid to prevent conflict with other tests
		uuid := stringid.GenerateNonCryptoID()
		secondPath := filepath.Join(cniPath, fmt.Sprintf("%s.conflist", uuid))
		writeConf([]byte(secondConf), secondPath)
		defer removeConf(secondPath)

		session := podmanTest.Podman([]string{"network", "inspect", "podman-integrationtest", "podman"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.IsJSONOutputValid()).To(BeTrue())
	})

	It("podman network inspect", func() {
		// Setup, use uuid to prevent conflict with other tests
		uuid := stringid.GenerateNonCryptoID()
		secondPath := filepath.Join(cniPath, fmt.Sprintf("%s.conflist", uuid))
		writeConf([]byte(secondConf), secondPath)
		defer removeConf(secondPath)

		session := podmanTest.Podman([]string{"network", "inspect", "podman-integrationtest", "--format", "{{.cniVersion}}"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputContains("0.3.0")).To(BeTrue())
	})

	It("podman inspect container single CNI network", func() {
		netName := "testNetSingleCNI"
		network := podmanTest.Podman([]string{"network", "create", "--subnet", "10.50.50.0/24", netName})
		network.WaitWithDefaultTimeout()
		Expect(network.ExitCode()).To(BeZero())
		defer podmanTest.removeCNINetwork(netName)

		ctrName := "testCtr"
		container := podmanTest.Podman([]string{"run", "-dt", "--network", netName, "--name", ctrName, ALPINE, "top"})
		container.WaitWithDefaultTimeout()
		Expect(container.ExitCode()).To(BeZero())

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(BeZero())
		conData := inspect.InspectContainerToJSON()
		Expect(len(conData)).To(Equal(1))
		Expect(len(conData[0].NetworkSettings.Networks)).To(Equal(1))
		net, ok := conData[0].NetworkSettings.Networks[netName]
		Expect(ok).To(BeTrue())
		Expect(net.NetworkID).To(Equal(netName))
		Expect(net.IPPrefixLen).To(Equal(24))
		Expect(strings.HasPrefix(net.IPAddress, "10.50.50.")).To(BeTrue())

		// Necessary to ensure the CNI network is removed cleanly
		rmAll := podmanTest.Podman([]string{"rm", "-f", ctrName})
		rmAll.WaitWithDefaultTimeout()
		Expect(rmAll.ExitCode()).To(BeZero())
	})

	It("podman inspect container two CNI networks", func() {
		netName1 := "testNetTwoCNI1"
		network1 := podmanTest.Podman([]string{"network", "create", "--subnet", "10.50.51.0/25", netName1})
		network1.WaitWithDefaultTimeout()
		Expect(network1.ExitCode()).To(BeZero())
		defer podmanTest.removeCNINetwork(netName1)

		netName2 := "testNetTwoCNI2"
		network2 := podmanTest.Podman([]string{"network", "create", "--subnet", "10.50.51.128/26", netName2})
		network2.WaitWithDefaultTimeout()
		Expect(network2.ExitCode()).To(BeZero())
		defer podmanTest.removeCNINetwork(netName2)

		ctrName := "testCtr"
		container := podmanTest.Podman([]string{"run", "-dt", "--network", fmt.Sprintf("%s,%s", netName1, netName2), "--name", ctrName, ALPINE, "top"})
		container.WaitWithDefaultTimeout()
		Expect(container.ExitCode()).To(BeZero())

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(BeZero())
		conData := inspect.InspectContainerToJSON()
		Expect(len(conData)).To(Equal(1))
		Expect(len(conData[0].NetworkSettings.Networks)).To(Equal(2))
		net1, ok := conData[0].NetworkSettings.Networks[netName1]
		Expect(ok).To(BeTrue())
		Expect(net1.NetworkID).To(Equal(netName1))
		Expect(net1.IPPrefixLen).To(Equal(25))
		Expect(strings.HasPrefix(net1.IPAddress, "10.50.51.")).To(BeTrue())
		net2, ok := conData[0].NetworkSettings.Networks[netName2]
		Expect(ok).To(BeTrue())
		Expect(net2.NetworkID).To(Equal(netName2))
		Expect(net2.IPPrefixLen).To(Equal(26))
		Expect(strings.HasPrefix(net2.IPAddress, "10.50.51.")).To(BeTrue())

		// Necessary to ensure the CNI network is removed cleanly
		rmAll := podmanTest.Podman([]string{"rm", "-f", ctrName})
		rmAll.WaitWithDefaultTimeout()
		Expect(rmAll.ExitCode()).To(BeZero())
	})
})
