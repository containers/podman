package integration

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman commit", func() {
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

	It("podman commit container", func() {
		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		check := podmanTest.Podman([]string{"inspect", "foobar.com/test1-image:latest"})
		check.WaitWithDefaultTimeout()
		data := check.InspectImageJSON()
		Expect(StringInSlice("foobar.com/test1-image:latest", data[0].RepoTags)).To(BeTrue())
	})

	It("podman commit container with message", func() {
		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "-f", "docker", "--message", "testing-commit", "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		check := podmanTest.Podman([]string{"inspect", "foobar.com/test1-image:latest"})
		check.WaitWithDefaultTimeout()
		data := check.InspectImageJSON()
		Expect(data[0].Comment).To(Equal("testing-commit"))
	})

	It("podman commit container with author", func() {
		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "--author", "snoopy", "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		check := podmanTest.Podman([]string{"inspect", "foobar.com/test1-image:latest"})
		check.WaitWithDefaultTimeout()
		data := check.InspectImageJSON()
		Expect(data[0].Author).To(Equal("snoopy"))
	})

	It("podman commit container with change flag", func() {
		podmanTest.RestoreArtifact(fedoraMinimal)
		test := podmanTest.Podman([]string{"run", "--name", "test1", "-d", fedoraMinimal, "ls"})
		test.WaitWithDefaultTimeout()
		Expect(test.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "--change", "LABEL=image=blue", "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		check := podmanTest.Podman([]string{"inspect", "foobar.com/test1-image:latest"})
		check.WaitWithDefaultTimeout()
		data := check.InspectImageJSON()
		foundBlue := false
		for _, i := range data[0].Labels {
			if i == "blue" {
				foundBlue = true
				break
			}
		}
		Expect(foundBlue).To(Equal(true))
	})

	It("podman commit container with pause flag", func() {
		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "--pause=false", "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		check := podmanTest.Podman([]string{"inspect", "foobar.com/test1-image:latest"})
		check.WaitWithDefaultTimeout()
		Expect(check.ExitCode()).To(Equal(0))
	})

	It("podman commit with volume mounts", func() {
		s := podmanTest.Podman([]string{"run", "--name", "test1", "-v", "/tmp:/foo", "alpine", "date"})
		s.WaitWithDefaultTimeout()
		Expect(s.ExitCode()).To(Equal(0))

		c := podmanTest.Podman([]string{"commit", "test1", "newimage"})
		c.WaitWithDefaultTimeout()
		Expect(c.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", "newimage"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		image := inspect.InspectImageJSON()
		_, ok := image[0].ContainerConfig.Volumes["/tmp"]
		Expect(ok).To(BeTrue())

		r := podmanTest.Podman([]string{"run", "newimage"})
		r.WaitWithDefaultTimeout()
		Expect(r.ExitCode()).To(Equal(0))
	})

})
