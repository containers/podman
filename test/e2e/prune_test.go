package integration

import (
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var pruneImage = `
FROM  alpine:latest
LABEL RUN podman --version
RUN apk update
RUN apk add bash`

var _ = Describe("Podman prune", func() {
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
		podmanTest.SeedImages()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman container prune containers", func() {
		top := podmanTest.RunTopContainer("")
		top.WaitWithDefaultTimeout()
		Expect(top.ExitCode()).To(Equal(0))

		top = podmanTest.RunTopContainer("")
		top.WaitWithDefaultTimeout()
		Expect(top.ExitCode()).To(Equal(0))
		cid := top.OutputToString()

		stop := podmanTest.Podman([]string{"stop", cid})
		stop.WaitWithDefaultTimeout()
		Expect(stop.ExitCode()).To(Equal(0))

		prune := podmanTest.Podman([]string{"container", "prune", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune.ExitCode()).To(Equal(0))

		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
	})

	It("podman container prune after create containers", func() {
		create := podmanTest.Podman([]string{"create", "--name", "test", BB})
		create.WaitWithDefaultTimeout()
		Expect(create.ExitCode()).To(Equal(0))

		prune := podmanTest.Podman([]string{"container", "prune", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune.ExitCode()).To(Equal(0))

		Expect(podmanTest.NumberOfContainers()).To(Equal(0))
	})

	It("podman container prune after create & init containers", func() {
		create := podmanTest.Podman([]string{"create", "--name", "test", BB})
		create.WaitWithDefaultTimeout()
		Expect(create.ExitCode()).To(Equal(0))

		init := podmanTest.Podman([]string{"init", "test"})
		init.WaitWithDefaultTimeout()
		Expect(init.ExitCode()).To(Equal(0))

		prune := podmanTest.Podman([]string{"container", "prune", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune.ExitCode()).To(Equal(0))

		Expect(podmanTest.NumberOfContainers()).To(Equal(0))
	})

	It("podman image prune skip cache images", func() {
		SkipIfRemote()
		podmanTest.BuildImage(pruneImage, "alpine_bash:latest", "true")

		none := podmanTest.Podman([]string{"images", "-a"})
		none.WaitWithDefaultTimeout()
		Expect(none.ExitCode()).To(Equal(0))
		hasNone, _ := none.GrepString("<none>")
		Expect(hasNone).To(BeTrue())

		prune := podmanTest.Podman([]string{"image", "prune", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune.ExitCode()).To(Equal(0))

		after := podmanTest.Podman([]string{"images", "-a"})
		after.WaitWithDefaultTimeout()
		Expect(none.ExitCode()).To(Equal(0))
		hasNoneAfter, _ := after.GrepString("<none>")
		Expect(hasNoneAfter).To(BeTrue())
		Expect(len(after.OutputToStringArray()) > 1).To(BeTrue())
	})

	It("podman image prune dangling images", func() {
		SkipIfRemote()
		podmanTest.BuildImage(pruneImage, "alpine_bash:latest", "true")
		podmanTest.BuildImage(pruneImage, "alpine_bash:latest", "true")

		none := podmanTest.Podman([]string{"images", "-a"})
		none.WaitWithDefaultTimeout()
		Expect(none.ExitCode()).To(Equal(0))
		hasNone, result := none.GrepString("<none>")
		Expect(len(result)).To(Equal(2))
		Expect(hasNone).To(BeTrue())

		prune := podmanTest.Podman([]string{"image", "prune", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune.ExitCode()).To(Equal(0))

		after := podmanTest.Podman([]string{"images", "-a"})
		after.WaitWithDefaultTimeout()
		Expect(none.ExitCode()).To(Equal(0))
		hasNoneAfter, result := none.GrepString("<none>")
		Expect(hasNoneAfter).To(BeTrue())
		Expect(len(after.OutputToStringArray()) > 1).To(BeTrue())
		Expect(len(result) > 0).To(BeTrue())
	})

	It("podman image prune unused images", func() {
		podmanTest.RestoreAllArtifacts()
		prune := podmanTest.PodmanNoCache([]string{"image", "prune", "-af"})
		prune.WaitWithDefaultTimeout()
		Expect(prune.ExitCode()).To(Equal(0))

		images := podmanTest.PodmanNoCache([]string{"images", "-aq"})
		images.WaitWithDefaultTimeout()
		// all images are unused, so they all should be deleted!
		Expect(len(images.OutputToStringArray())).To(Equal(0))
	})

	It("podman system image prune unused images", func() {
		SkipIfRemote()
		podmanTest.RestoreAllArtifacts()
		podmanTest.BuildImage(pruneImage, "alpine_bash:latest", "true")
		prune := podmanTest.PodmanNoCache([]string{"system", "prune", "-a", "--force"})
		prune.WaitWithDefaultTimeout()
		Expect(prune.ExitCode()).To(Equal(0))

		images := podmanTest.PodmanNoCache([]string{"images", "-aq"})
		images.WaitWithDefaultTimeout()
		// all images are unused, so they all should be deleted!
		Expect(len(images.OutputToStringArray())).To(Equal(0))
	})

	It("podman system prune pods", func() {
		Skip(v2remotefail)
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "start", "-l"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "stop", "-l"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		pods := podmanTest.Podman([]string{"pod", "ps"})
		pods.WaitWithDefaultTimeout()
		Expect(pods.ExitCode()).To(Equal(0))
		Expect(len(pods.OutputToStringArray())).To(Equal(3))

		prune := podmanTest.Podman([]string{"system", "prune", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune.ExitCode()).To(Equal(0))

		pods = podmanTest.Podman([]string{"pod", "ps"})
		pods.WaitWithDefaultTimeout()
		Expect(pods.ExitCode()).To(Equal(0))
		Expect(len(pods.OutputToStringArray())).To(Equal(2))
	})

	It("podman system prune - pod,container stopped", func() {
		Skip(v2remotefail)
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Start and stop a pod to get it in exited state.
		session = podmanTest.Podman([]string{"pod", "start", "-l"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "stop", "-l"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Create a container. This container should be pruned.
		create := podmanTest.Podman([]string{"create", "--name", "test", BB})
		create.WaitWithDefaultTimeout()
		Expect(create.ExitCode()).To(Equal(0))

		prune := podmanTest.Podman([]string{"system", "prune", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune.ExitCode()).To(Equal(0))

		pods := podmanTest.Podman([]string{"pod", "ps"})
		pods.WaitWithDefaultTimeout()
		Expect(pods.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfPods()).To(Equal(0))

		Expect(podmanTest.NumberOfContainers()).To(Equal(0))
	})

	It("podman system prune with running, exited pod and volume prune set true", func() {
		Skip(v2remotefail)
		// Start and stop a pod to get it in exited state.
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "start", "-l"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "stop", "-l"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Start a pod and leave it running
		session = podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"pod", "start", "-l"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Number of pod should be 2. One exited one running.
		Expect(podmanTest.NumberOfPods()).To(Equal(2))

		// Create a container. This container should be pruned.
		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))

		// Number of containers should be three now.
		// Two as pods infra container and one newly created.
		Expect(podmanTest.NumberOfContainers()).To(Equal(3))

		// image list current count should not be pruned if all flag isnt enabled
		session = podmanTest.Podman([]string{"images"})
		session.WaitWithDefaultTimeout()
		numberOfImages := len(session.OutputToStringArray())

		// Adding unused volume should be pruned
		session = podmanTest.Podman([]string{"volume", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"create", "-v", "myvol:/myvol", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(3))

		session = podmanTest.Podman([]string{"system", "prune", "--force", "--volumes"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Volumes should be pruned.
		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(0))

		// One Pod should not be pruned as it was running
		Expect(podmanTest.NumberOfPods()).To(Equal(1))

		// Running pods infra container should not be pruned.
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		// Image should not be pruned and number should be same.
		images := podmanTest.Podman([]string{"images"})
		images.WaitWithDefaultTimeout()
		Expect(len(images.OutputToStringArray())).To(Equal(numberOfImages))
	})

	It("podman system prune - with dangling images true", func() {
		Skip(v2remotefail)
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Start and stop a pod to get it in exited state.
		session = podmanTest.Podman([]string{"pod", "start", "-l"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "stop", "-l"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Create a container. This container should be pruned.
		create := podmanTest.Podman([]string{"create", "--name", "test", BB})
		create.WaitWithDefaultTimeout()
		Expect(create.ExitCode()).To(Equal(0))

		// Adding images should be pruned
		podmanTest.BuildImage(pruneImage, "alpine_bash:latest", "true")

		// Adding unused volume should not be pruned as volumes not set
		session = podmanTest.Podman([]string{"volume", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		prune := podmanTest.Podman([]string{"system", "prune", "-f", "-a"})
		prune.WaitWithDefaultTimeout()
		Expect(prune.ExitCode()).To(Equal(0))

		pods := podmanTest.Podman([]string{"pod", "ps"})
		pods.WaitWithDefaultTimeout()
		Expect(pods.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfPods()).To(Equal(0))

		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Volumes should not be pruned
		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(2))

		images := podmanTest.PodmanNoCache([]string{"images", "-aq"})
		images.WaitWithDefaultTimeout()
		// all images are unused, so they all should be deleted!
		Expect(len(images.OutputToStringArray())).To(Equal(0))
	})
})
