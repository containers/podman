package integration

import (
	"fmt"
	"os"

	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var pruneImage = fmt.Sprintf(`
FROM  %s
LABEL RUN podman --version
RUN apk update
RUN apk add bash`, ALPINE)

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

	It("podman image prune - remove only dangling images", func() {
		session := podmanTest.Podman([]string{"images", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		hasNone, _ := session.GrepString("<none>")
		Expect(hasNone).To(BeFalse())
		numImages := len(session.OutputToStringArray())

		// Since there's no dangling image, none should be removed.
		session = podmanTest.Podman([]string{"image", "prune", "-f"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(0))

		// Let's be extra sure that the same number of images is
		// reported.
		session = podmanTest.Podman([]string{"images", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(numImages))

		// Now build a new image with dangling intermediate images.
		podmanTest.BuildImage(pruneImage, "alpine_bash:latest", "true")

		session = podmanTest.Podman([]string{"images", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		hasNone, _ = session.GrepString("<none>")
		Expect(hasNone).To(BeTrue()) // ! we have dangling ones
		numImages = len(session.OutputToStringArray())

		// Since there's at least one dangling image, prune should
		// remove them.
		session = podmanTest.Podman([]string{"image", "prune", "-f"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		numPrunedImages := len(session.OutputToStringArray())
		Expect(numPrunedImages >= 1).To(BeTrue())

		// Now make sure that exactly the number of pruned images has
		// been removed.
		session = podmanTest.Podman([]string{"images", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(numImages - numPrunedImages))
	})

	It("podman image prune skip cache images", func() {
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
		// Check if all "dangling" images were pruned.
		hasNoneAfter, _ := after.GrepString("<none>")
		Expect(hasNoneAfter).To(BeFalse())
		Expect(len(after.OutputToStringArray()) > 1).To(BeTrue())
	})

	It("podman image prune dangling images", func() {
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
		podmanTest.AddImageToRWStore(ALPINE)
		podmanTest.AddImageToRWStore(BB)

		images := podmanTest.Podman([]string{"images", "-a"})
		images.WaitWithDefaultTimeout()
		Expect(images.ExitCode()).To(Equal(0))

		prune := podmanTest.Podman([]string{"image", "prune", "-af"})
		prune.WaitWithDefaultTimeout()
		Expect(prune.ExitCode()).To(Equal(0))

		images = podmanTest.Podman([]string{"images", "-aq"})
		images.WaitWithDefaultTimeout()
		Expect(images.ExitCode()).To(Equal(0))
		// all images are unused, so they all should be deleted!
		Expect(len(images.OutputToStringArray())).To(Equal(len(CACHE_IMAGES)))
	})

	It("podman system image prune unused images", func() {
		podmanTest.AddImageToRWStore(ALPINE)
		podmanTest.BuildImage(pruneImage, "alpine_bash:latest", "true")
		prune := podmanTest.Podman([]string{"system", "prune", "-a", "--force"})
		prune.WaitWithDefaultTimeout()
		Expect(prune.ExitCode()).To(Equal(0))

		images := podmanTest.Podman([]string{"images", "-aq"})
		images.WaitWithDefaultTimeout()
		// all images are unused, so they all should be deleted!
		Expect(len(images.OutputToStringArray())).To(Equal(len(CACHE_IMAGES)))
	})

	It("podman system prune pods", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podid1 := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podid1})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "stop", podid1})
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
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podid1 := session.OutputToString()

		// Start and stop a pod to get it in exited state.
		session = podmanTest.Podman([]string{"pod", "start", podid1})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "stop", podid1})
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
		// Start and stop a pod to get it in exited state.
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podid1 := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podid1})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "stop", podid1})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Start a pod and leave it running
		session = podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podid2 := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podid2})
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

		// image list current count should not be pruned if all flag isn't enabled
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
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podid1 := session.OutputToString()

		// Start and stop a pod to get it in exited state.
		session = podmanTest.Podman([]string{"pod", "start", podid1})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "stop", podid1})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Create a container. This container should be pruned.
		create := podmanTest.Podman([]string{"create", "--name", "test", BB})
		create.WaitWithDefaultTimeout()
		Expect(create.ExitCode()).To(Equal(0))

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

		images := podmanTest.Podman([]string{"images", "-aq"})
		images.WaitWithDefaultTimeout()
		// all images are unused, so they all should be deleted!
		Expect(len(images.OutputToStringArray())).To(Equal(len(CACHE_IMAGES)))
	})

	It("podman system prune --volumes --filter", func() {
		session := podmanTest.Podman([]string{"volume", "create", "--label", "label1=value1", "myvol1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"volume", "create", "--label", "sharedlabel1=slv1", "myvol2"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"volume", "create", "--label", "sharedlabel1=slv2", "myvol3"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"volume", "create", "--label", "sharedlabel1", "myvol4"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"create", "-v", "myvol5:/myvol5", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"create", "-v", "myvol6:/myvol6", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(7))

		session = podmanTest.Podman([]string{"system", "prune", "--force", "--volumes", "--filter", "label=label1=value1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(6))

		session = podmanTest.Podman([]string{"system", "prune", "--force", "--volumes", "--filter", "label=sharedlabel1=slv1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(5))

		session = podmanTest.Podman([]string{"system", "prune", "--force", "--volumes", "--filter", "label=sharedlabel1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(3))

		podmanTest.Cleanup()
	})
})
