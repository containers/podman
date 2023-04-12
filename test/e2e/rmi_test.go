package integration

import (
	"fmt"
	"os"
	"sync"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
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
		podmanTest.AddImageToRWStore(ALPINE)
		session := podmanTest.Podman([]string{"rmi", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

	})

	It("podman rmi with short name", func() {
		podmanTest.AddImageToRWStore(CIRROS_IMAGE)
		session := podmanTest.Podman([]string{"rmi", "cirros"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

	})

	It("podman rmi all images", func() {
		podmanTest.AddImageToRWStore(NGINX_IMAGE)
		session := podmanTest.Podman([]string{"rmi", "-a"})
		session.WaitWithDefaultTimeout()
		images := podmanTest.Podman([]string{"images"})
		images.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

	})

	It("podman rmi all images forcibly with short options", func() {
		podmanTest.AddImageToRWStore(NGINX_IMAGE)
		session := podmanTest.Podman([]string{"rmi", "-fa"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

	})

	It("podman rmi tagged image", func() {
		podmanTest.AddImageToRWStore(CIRROS_IMAGE)
		setup := podmanTest.Podman([]string{"images", "-q", CIRROS_IMAGE})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session := podmanTest.Podman([]string{"tag", CIRROS_IMAGE, "foo:bar", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		result := podmanTest.Podman([]string{"images", "-q", "foo"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		Expect(result.OutputToString()).To(ContainSubstring(setup.OutputToString()))
	})

	It("podman rmi image with tags by ID cannot be done without force", func() {
		podmanTest.AddImageToRWStore(CIRROS_IMAGE)
		setup := podmanTest.Podman([]string{"images", "-q", CIRROS_IMAGE})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))
		cirrosID := setup.OutputToString()

		session := podmanTest.Podman([]string{"tag", "cirros", "foo:bar", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Trying without --force should fail
		result := podmanTest.Podman([]string{"rmi", cirrosID})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError())

		// With --force it should work
		resultForce := podmanTest.Podman([]string{"rmi", "-f", cirrosID})
		resultForce.WaitWithDefaultTimeout()
		Expect(resultForce).Should(Exit(0))
	})

	It("podman rmi image that is a parent of another image", func() {
		Skip("I need help with this one. i don't understand what is going on")
		podmanTest.AddImageToRWStore(CIRROS_IMAGE)
		session := podmanTest.Podman([]string{"run", "--name", "c_test", CIRROS_IMAGE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"commit", "-q", "c_test", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"rm", "c_test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"rmi", CIRROS_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"images", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(12))

		session = podmanTest.Podman([]string{"images", "--sort", "created", "--format", "{{.Id}}", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(13),
			"Output from 'podman images -q -a'")
		untaggedImg := session.OutputToStringArray()[1]

		session = podmanTest.Podman([]string{"rmi", "-f", untaggedImg})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(2), "UntaggedImg is '%s'", untaggedImg)
	})

	It("podman rmi image that is created from another named imaged", func() {
		podmanTest.AddImageToRWStore(ALPINE)
		session := podmanTest.Podman([]string{"create", "--name", "c_test1", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"commit", "-q", "c_test1", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"create", "--name", "c_test2", "test1", "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"commit", "-q", "c_test2", "test2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"rm", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"rmi", "test2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"images", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(len(CACHE_IMAGES) + 1))
	})

	It("podman rmi with cached images", func() {
		podmanTest.AddImageToRWStore(CIRROS_IMAGE)
		dockerfile := fmt.Sprintf(`FROM %s
		RUN mkdir hello
		RUN touch test.txt
		ENV foo=bar
		`, CIRROS_IMAGE)
		podmanTest.BuildImage(dockerfile, "test", "true")

		dockerfile = fmt.Sprintf(`FROM %s
		RUN mkdir hello
		RUN touch test.txt
		RUN mkdir blah
		ENV foo=bar
		`, CIRROS_IMAGE)

		podmanTest.BuildImage(dockerfile, "test2", "true")

		session := podmanTest.Podman([]string{"images", "-q", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		numOfImages := len(session.OutputToStringArray())

		session = podmanTest.Podman([]string{"rmi", "test2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"images", "-q", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(numOfImages - len(session.OutputToStringArray())).To(Equal(2))

		session = podmanTest.Podman([]string{"rmi", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"images", "-q", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(len(CACHE_IMAGES) + 1))

		podmanTest.BuildImage(dockerfile, "test3", "true")

		session = podmanTest.Podman([]string{"rmi", CIRROS_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"rmi", "test3"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"images", "-q", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(HaveLen(142))
	})

	It("podman rmi -a with no images should be exit 0", func() {
		session := podmanTest.Podman([]string{"rmi", "-fa"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session2 := podmanTest.Podman([]string{"rmi", "-fa"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))
	})

	It("podman rmi -a with parent|child images", func() {
		podmanTest.AddImageToRWStore(CIRROS_IMAGE)
		dockerfile := fmt.Sprintf(`FROM %s AS base
RUN touch /1
ENV LOCAL=/1
RUN find $LOCAL
FROM base
RUN find $LOCAL

`, CIRROS_IMAGE)
		podmanTest.BuildImage(dockerfile, "test", "true")
		session := podmanTest.Podman([]string{"rmi", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		images := podmanTest.Podman([]string{"images", "-aq"})
		images.WaitWithDefaultTimeout()
		Expect(images).Should(Exit(0))
		Expect(images.OutputToStringArray()).To(HaveLen(len(CACHE_IMAGES)))
	})

	// Don't rerun all tests; just assume that if we get that diagnostic,
	// we're getting rmi
	It("podman image rm is the same as rmi", func() {
		session := podmanTest.Podman([]string{"image", "rm"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		Expect(session.ErrorToString()).To(ContainSubstring("image name or ID must be specified"))
	})

	It("podman image rm - concurrent with shared layers", func() {
		// #6510 has shown a fairly simple reproducer to force storage
		// errors during parallel image removal.  Since it's subject to
		// a race, we may not hit the condition a 100 percent of times
		// but ocal reproducers hit it all the time.

		podmanTest.AddImageToRWStore(CIRROS_IMAGE)
		var wg sync.WaitGroup

		buildAndRemove := func(i int) {
			defer GinkgoRecover()
			defer wg.Done()
			imageName := fmt.Sprintf("rmtest:%d", i)
			containerfile := fmt.Sprintf(`FROM %s
RUN touch %s`, CIRROS_IMAGE, imageName)

			podmanTest.BuildImage(containerfile, imageName, "false")
			session := podmanTest.Podman([]string{"rmi", "-f", imageName})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
		}

		wg.Add(10)
		for i := 0; i < 10; i++ {
			go buildAndRemove(i)
		}
		wg.Wait()
	})

	It("podman rmi --no-prune with dangling parents", func() {
		podmanTest.AddImageToRWStore(ALPINE)
		session := podmanTest.Podman([]string{"create", "--name", "c_test1", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"commit", "-q", "c_test1", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"create", "--name", "c_test2", "test1", "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"commit", "-q", "c_test2", "test2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		imageID2 := session.OutputToString()

		session = podmanTest.Podman([]string{"create", "--name", "c_test3", "test2", "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"commit", "-q", "c_test3", "test3"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		imageID3 := session.OutputToString()

		session = podmanTest.Podman([]string{"untag", "test2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"untag", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"rmi", "-f", "--no-prune", "test3"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(imageID3))
		Expect(session.OutputToString()).NotTo(ContainSubstring(imageID2))
	})

	It("podman rmi --no-prune with undangling parents", func() {
		podmanTest.AddImageToRWStore(ALPINE)
		session := podmanTest.Podman([]string{"create", "--name", "c_test1", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"commit", "-q", "c_test1", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"create", "--name", "c_test2", "test1", "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"commit", "-q", "c_test2", "test2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		imageID2 := session.OutputToString()

		session = podmanTest.Podman([]string{"create", "--name", "c_test3", "test2", "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"commit", "-q", "c_test3", "test3"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		imageID3 := session.OutputToString()

		session = podmanTest.Podman([]string{"rmi", "-f", "--no-prune", "test3"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(imageID3))
		Expect(session.OutputToString()).NotTo(ContainSubstring(imageID2))
	})
})
