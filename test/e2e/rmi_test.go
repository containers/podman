package integration

import (
	"fmt"
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman rmi", func() {
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
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman rmi bogus image", func() {
		session := podmanTest.Podman([]string{"rmi", "debian:6.0.10"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))

	})

	It("podman rmi with fq name", func() {
		session := podmanTest.PodmanNoCache([]string{"rmi", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

	})

	It("podman rmi with short name", func() {
		session := podmanTest.PodmanNoCache([]string{"rmi", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

	})

	It("podman rmi all images", func() {
		podmanTest.RestoreArtifact(nginx)
		session := podmanTest.PodmanNoCache([]string{"rmi", "-a"})
		session.WaitWithDefaultTimeout()
		images := podmanTest.PodmanNoCache([]string{"images"})
		images.WaitWithDefaultTimeout()
		fmt.Println(images.OutputToStringArray())
		Expect(session).Should(Exit(0))

	})

	It("podman rmi all images forcibly with short options", func() {
		podmanTest.RestoreArtifact(nginx)
		session := podmanTest.PodmanNoCache([]string{"rmi", "-fa"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

	})

	It("podman rmi tagged image", func() {
		setup := podmanTest.PodmanNoCache([]string{"images", "-q", ALPINE})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session := podmanTest.PodmanNoCache([]string{"tag", "alpine", "foo:bar", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		result := podmanTest.PodmanNoCache([]string{"images", "-q", "foo"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		Expect(result.LineInOutputContains(setup.OutputToString())).To(BeTrue())
	})

	It("podman rmi image with tags by ID cannot be done without force", func() {
		setup := podmanTest.PodmanNoCache([]string{"images", "-q", ALPINE})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))
		alpineId := setup.OutputToString()

		session := podmanTest.PodmanNoCache([]string{"tag", "alpine", "foo:bar", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Trying without --force should fail
		result := podmanTest.PodmanNoCache([]string{"rmi", alpineId})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError())

		// With --force it should work
		resultForce := podmanTest.PodmanNoCache([]string{"rmi", "-f", alpineId})
		resultForce.WaitWithDefaultTimeout()
		Expect(resultForce).Should(Exit(0))
	})

	It("podman rmi image that is a parent of another image", func() {
		session := podmanTest.PodmanNoCache([]string{"rmi", "-fa"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.PodmanNoCache([]string{"run", "--name", "c_test", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.PodmanNoCache([]string{"commit", "-q", "c_test", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.PodmanNoCache([]string{"rm", "c_test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.PodmanNoCache([]string{"rmi", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.PodmanNoCache([]string{"images", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(len(session.OutputToStringArray())).To(Equal(1))

		session = podmanTest.PodmanNoCache([]string{"images", "-q", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(len(session.OutputToStringArray())).To(Equal(2),
			"Output from 'podman images -q -a':'%s'", session.Out.Contents())
		untaggedImg := session.OutputToStringArray()[0]

		session = podmanTest.PodmanNoCache([]string{"rmi", "-f", untaggedImg})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(2), "UntaggedImg is '%s'", untaggedImg)
	})

	It("podman rmi image that is created from another named imaged", func() {
		session := podmanTest.PodmanNoCache([]string{"rmi", "-fa"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.PodmanNoCache([]string{"create", "--name", "c_test1", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.PodmanNoCache([]string{"commit", "-q", "c_test1", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.PodmanNoCache([]string{"create", "--name", "c_test2", "test1", "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.PodmanNoCache([]string{"commit", "-q", "c_test2", "test2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.PodmanNoCache([]string{"rm", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.PodmanNoCache([]string{"rmi", "test2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.PodmanNoCache([]string{"images", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(len(session.OutputToStringArray())).To(Equal(2))
	})

	It("podman rmi with cached images", func() {
		SkipIfRemote()
		session := podmanTest.PodmanNoCache([]string{"rmi", "-fa"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

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

		session = podmanTest.PodmanNoCache([]string{"images", "-q", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		numOfImages := len(session.OutputToStringArray())

		session = podmanTest.PodmanNoCache([]string{"rmi", "test2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.PodmanNoCache([]string{"images", "-q", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(numOfImages - len(session.OutputToStringArray())).To(Equal(2))

		session = podmanTest.PodmanNoCache([]string{"rmi", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.PodmanNoCache([]string{"images", "-q", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(len(session.OutputToStringArray())).To(Equal(1))

		podmanTest.BuildImage(dockerfile, "test3", "true")

		session = podmanTest.PodmanNoCache([]string{"rmi", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.PodmanNoCache([]string{"rmi", "test3"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.PodmanNoCache([]string{"images", "-q", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(len(session.OutputToString())).To(Equal(0))
	})

	It("podman rmi -a with no images should be exit 0", func() {
		session := podmanTest.PodmanNoCache([]string{"rmi", "-fa"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session2 := podmanTest.PodmanNoCache([]string{"rmi", "-fa"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))
	})

	It("podman rmi -a with parent|child images", func() {
		SkipIfRemote()
		dockerfile := `FROM docker.io/library/alpine:latest AS base
RUN touch /1
ENV LOCAL=/1
RUN find $LOCAL
FROM base
RUN find $LOCAL

`
		podmanTest.BuildImage(dockerfile, "test", "true")
		session := podmanTest.PodmanNoCache([]string{"rmi", "-a"})
		session.WaitWithDefaultTimeout()
		fmt.Println(session.OutputToString())
		Expect(session).Should(Exit(0))

		images := podmanTest.PodmanNoCache([]string{"images", "-aq"})
		images.WaitWithDefaultTimeout()
		Expect(images).Should(Exit(0))
		Expect(len(images.OutputToStringArray())).To(Equal(0))
	})

	// Don't rerun all tests; just assume that if we get that diagnostic,
	// we're getting rmi
	It("podman image rm is the same as rmi", func() {
		session := podmanTest.PodmanNoCache([]string{"image", "rm"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		match, _ := session.ErrorGrepString("image name or ID must be specified")
		Expect(match).To(BeTrue())
	})
})
