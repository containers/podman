package integration

import (
	"fmt"
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman pod create", func() {
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
		podmanTest.RestoreAllArtifacts()
		podmanTest.RestoreArtifact(infra)
	})

	AfterEach(func() {
		podmanTest.CleanupPod()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
	})

	It("podman pod container share Namespaces", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", "--pod", podID, "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		check := podmanTest.Podman([]string{"ps", "-a", "--ns", "--format", "{{.IPC}} {{.UTS}} {{.NET}}"})
		check.WaitWithDefaultTimeout()
		Expect(check.ExitCode()).To(Equal(0))
		outputArray := check.OutputToStringArray()
		Expect(len(outputArray)).To(Equal(2))

		NAMESPACE1 := outputArray[0]
		fmt.Println("NAMESPACE1:", NAMESPACE1)
		NAMESPACE2 := outputArray[1]
		fmt.Println("NAMESPACE2:", NAMESPACE2)
		Expect(NAMESPACE1).To(Equal(NAMESPACE2))
	})

	It("podman pod container dontshare PIDNS", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", "--pod", podID, "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		check := podmanTest.Podman([]string{"ps", "-a", "--ns", "--format", "{{.PIDNS}}"})
		check.WaitWithDefaultTimeout()
		Expect(check.ExitCode()).To(Equal(0))
		outputArray := check.OutputToStringArray()
		Expect(len(outputArray)).To(Equal(2))

		NAMESPACE1 := outputArray[0]
		fmt.Println("NAMESPACE1:", NAMESPACE1)
		NAMESPACE2 := outputArray[1]
		fmt.Println("NAMESPACE2:", NAMESPACE2)
		Expect(NAMESPACE1).To(Not(Equal(NAMESPACE2)))
	})

})
