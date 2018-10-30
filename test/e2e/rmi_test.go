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

	It("podman rmi bogus image", func() {
		session := podmanTest.Podman([]string{"rmi", "debian:6.0.10"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))

	})

	It("podman rmi with fq name", func() {
		session := podmanTest.Podman([]string{"rmi", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

	})

	It("podman rmi with short name", func() {
		session := podmanTest.Podman([]string{"rmi", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

	})

	It("podman rmi all images", func() {
		podmanTest.PullImages([]string{nginx})
		session := podmanTest.Podman([]string{"rmi", "-a"})
		session.WaitWithDefaultTimeout()
		images := podmanTest.Podman([]string{"images"})
		images.WaitWithDefaultTimeout()
		fmt.Println(images.OutputToStringArray())
		Expect(session.ExitCode()).To(Equal(0))

	})

	It("podman rmi all images forcibly with short options", func() {
		podmanTest.PullImages([]string{nginx})
		session := podmanTest.Podman([]string{"rmi", "-fa"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

	})

	It("podman rmi tagged image", func() {
		setup := podmanTest.Podman([]string{"images", "-q", ALPINE})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"tag", "alpine", "foo:bar", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"images", "-q", "foo"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		Expect(result.LineInOutputContains(setup.OutputToString())).To(BeTrue())
	})

	It("podman rmi image with tags by ID cannot be done without force", func() {
		setup := podmanTest.Podman([]string{"images", "-q", ALPINE})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))
		alpineId := setup.OutputToString()

		session := podmanTest.Podman([]string{"tag", "alpine", "foo:bar", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Trying without --force should fail
		result := podmanTest.Podman([]string{"rmi", alpineId})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).ToNot(Equal(0))

		// With --force it should work
		resultForce := podmanTest.Podman([]string{"rmi", "-f", alpineId})
		resultForce.WaitWithDefaultTimeout()
		Expect(resultForce.ExitCode()).To(Equal(0))
	})

	It("podman rmi image that is a parent of another image", func() {
		session := podmanTest.Podman([]string{"rmi", "-fa"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", "--name", "c_test", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"commit", "-q", "c_test", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"rm", "c_test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"rmi", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"images", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(1))

		session = podmanTest.Podman([]string{"images", "-q", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(2))
		untaggedImg := session.OutputToStringArray()[1]

		session = podmanTest.Podman([]string{"rmi", "-f", untaggedImg})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
	})

	It("podman rmi image that is created from another named imaged", func() {
		session := podmanTest.Podman([]string{"rmi", "-fa"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"create", "--name", "c_test1", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"commit", "-q", "c_test1", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"create", "--name", "c_test2", "test1", "true"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"commit", "-q", "c_test2", "test2"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"rm", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"rmi", "test2"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"images", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(2))
	})

	It("podman rmi with cached images", func() {
		session := podmanTest.Podman([]string{"rmi", "-fa"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		dockerfile := `FROM docker.io/library/alpine:latest
		RUN mkdir hello
		RUN touch test.txt
		ENV foo=bar
		`
		podmanTest.BuildImage(dockerfile, "test", "true")

		dockerfile = `FROM docker.io/library/alpine:latest
		RUN mkdir hello
		RUN touch test.txt
		RUN mkdir blah
		ENV foo=bar
		`
		podmanTest.BuildImage(dockerfile, "test2", "true")

		session = podmanTest.Podman([]string{"images", "-q", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		numOfImages := len(session.OutputToStringArray())

		session = podmanTest.Podman([]string{"rmi", "test2"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"images", "-q", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(numOfImages - len(session.OutputToStringArray())).To(Equal(2))

		session = podmanTest.Podman([]string{"rmi", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"images", "-q", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(1))

		podmanTest.BuildImage(dockerfile, "test3", "true")

		session = podmanTest.Podman([]string{"rmi", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"rmi", "test3"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"images", "-q", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToString())).To(Equal(0))
	})

	It("podman rmi -a with no images should be exit 0", func() {
		session := podmanTest.Podman([]string{"rmi", "-fa"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session2 := podmanTest.Podman([]string{"rmi", "-fa"})
		session2.WaitWithDefaultTimeout()
		Expect(session2.ExitCode()).To(Equal(0))
	})
})
