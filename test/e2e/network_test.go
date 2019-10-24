// +build !remoteclient

package integration

import (
	"fmt"
	. "github.com/containers/libpod/test/utils"
	"github.com/containers/storage/pkg/stringid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"os"
	"path/filepath"
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
		SkipIfRootless()
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
		SkipIfRootless()
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

	It("podman network rm no args", func() {
		SkipIfRootless()
		session := podmanTest.Podman([]string{"network", "rm"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).ToNot(BeZero())
	})

	It("podman network rm", func() {
		SkipIfRootless()
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
		SkipIfRootless()
		session := podmanTest.Podman([]string{"network", "inspect"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).ToNot(BeZero())
	})

	It("podman network inspect", func() {
		SkipIfRootless()
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

	It("podman network attach to rootless", func() {
		SkipIfRoot()
		// Setup, use uuid to prevent conflict with other tests
		uuid := stringid.GenerateNonCryptoID()
		secondPath := filepath.Join(cniPath, fmt.Sprintf("%s.conflist", uuid))
		writeConf([]byte(secondConf), secondPath)
		defer removeConf(secondPath)

		session := podmanTest.Podman([]string{"run", "--network", "podman-integrationtest", ALPINE, "sh", "-c", "ip addr"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
		Expect(session.LineInOutputContains("eth0")).To(BeFalse())
		Expect(session.LineInOutputContains("Error: cannot use CNI networks with rootless containers")).To(BeTrue())
	})

})
