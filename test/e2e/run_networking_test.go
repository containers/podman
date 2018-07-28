package integration

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman rmi", func() {
	var (
		tempdir    string
		err        error
		podmanTest PodmanTest
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanCreate(tempdir)
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
	})

	It("podman run network connection with default bridge", func() {
		session := podmanTest.Podman([]string{"run", "-dt", ALPINE, "wget", "www.projectatomic.io"})
		session.Wait(90)
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run network connection with host", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--network", "host", ALPINE, "wget", "www.projectatomic.io"})
		session.Wait(90)
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run network connection with loopback", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--network", "host", ALPINE, "wget", "www.projectatomic.io"})
		session.Wait(90)
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run network expose port 222", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--expose", "222-223", "-P", ALPINE, "/bin/sh"})
		session.Wait(30)
		Expect(session.ExitCode()).To(Equal(0))
		results := podmanTest.SystemExec("iptables", []string{"-t", "nat", "-L"})
		results.Wait(30)
		Expect(results.ExitCode()).To(Equal(0))
		Expect(results.OutputToString()).To(ContainSubstring("222"))
		Expect(results.OutputToString()).To(ContainSubstring("223"))
	})

	It("podman run network expose host port 80 to container port 8000", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "-p", "80:8000", ALPINE, "/bin/sh"})
		session.Wait(30)
		Expect(session.ExitCode()).To(Equal(0))
		results := podmanTest.SystemExec("iptables", []string{"-t", "nat", "-L"})
		results.Wait(30)
		Expect(results.ExitCode()).To(Equal(0))
		Expect(results.OutputToString()).To(ContainSubstring("8000"))

		ncBusy := podmanTest.SystemExec("nc", []string{"-l", "-p", "80"})
		ncBusy.Wait(10)
		Expect(ncBusy.ExitCode()).ToNot(Equal(0))
	})

	It("podman run network expose ports in image metadata", func() {
		podmanTest.RestoreArtifact(nginx)
		session := podmanTest.Podman([]string{"run", "-dt", "-P", nginx})
		session.Wait(90)
		Expect(session.ExitCode()).To(Equal(0))
		results := podmanTest.Podman([]string{"inspect", "-l"})
		results.Wait(30)
		Expect(results.ExitCode()).To(Equal(0))
		Expect(results.OutputToString()).To(ContainSubstring(": 80,"))
	})

	It("podman run network expose duplicate host port results in error", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "-p", "80", ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", "-l"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))

		containerConfig := inspect.InspectContainerToJSON()
		Expect(containerConfig[0].NetworkSettings.Ports[0].HostPort).ToNot(Equal("80"))
	})

})
