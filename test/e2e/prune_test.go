package integration

import (
	"fmt"
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var pruneImage = fmt.Sprintf(`
FROM  %s
LABEL RUN podman --version
RUN apk update
RUN apk add bash`, ALPINE)

var emptyPruneImage = `
FROM scratch
ENV test1=test1
ENV test2=test2`

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
		Expect(top).Should(Exit(0))

		top = podmanTest.RunTopContainer("")
		top.WaitWithDefaultTimeout()
		Expect(top).Should(Exit(0))
		cid := top.OutputToString()

		stop := podmanTest.Podman([]string{"stop", cid})
		stop.WaitWithDefaultTimeout()
		Expect(stop).Should(Exit(0))

		prune := podmanTest.Podman([]string{"container", "prune", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(Exit(0))

		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
	})

	It("podman container prune after create containers", func() {
		create := podmanTest.Podman([]string{"create", "--name", "test", BB})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		prune := podmanTest.Podman([]string{"container", "prune", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(Exit(0))

		Expect(podmanTest.NumberOfContainers()).To(Equal(0))
	})

	It("podman container prune after create & init containers", func() {
		create := podmanTest.Podman([]string{"create", "--name", "test", BB})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		init := podmanTest.Podman([]string{"init", "test"})
		init.WaitWithDefaultTimeout()
		Expect(init).Should(Exit(0))

		prune := podmanTest.Podman([]string{"container", "prune", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(Exit(0))

		Expect(podmanTest.NumberOfContainers()).To(Equal(0))
	})

	It("podman image prune - remove only dangling images", func() {
		session := podmanTest.Podman([]string{"images", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Not(ContainSubstring("<none>")))
		numImages := len(session.OutputToStringArray())

		// Since there's no dangling image, none should be removed.
		session = podmanTest.Podman([]string{"image", "prune", "-f"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(BeEmpty())

		// Let's be extra sure that the same number of images is
		// reported.
		session = podmanTest.Podman([]string{"images", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(numImages))

		// Now build an image and untag it.  The (intermediate) images
		// should be removed recursively during pruning.
		podmanTest.BuildImage(pruneImage, "alpine_bash:latest", "true")
		session = podmanTest.Podman([]string{"untag", "alpine_bash:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"images", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("<none>"))
		numImages = len(session.OutputToStringArray())

		// Since there's at least one dangling image, prune should
		// remove them.
		session = podmanTest.Podman([]string{"image", "prune", "-f"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		numPrunedImages := len(session.OutputToStringArray())
		Expect(numPrunedImages).To(BeNumerically(">=", 1), "numPrunedImages")

		// Now make sure that exactly the number of pruned images has
		// been removed.
		session = podmanTest.Podman([]string{"images", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(numImages - numPrunedImages))
	})

	It("podman image prune - handle empty images", func() {
		// As shown in #10832, empty images were not treated correctly
		// in Podman.
		podmanTest.BuildImage(emptyPruneImage, "empty:scratch", "true")

		session := podmanTest.Podman([]string{"images", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("<none>"))

		// Nothing will be pruned.
		session = podmanTest.Podman([]string{"image", "prune", "-f"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(BeEmpty())

		// Now the image will be untagged, and its parent images will
		// be removed recursively.
		session = podmanTest.Podman([]string{"untag", "empty:scratch"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"image", "prune", "-f"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(2))
	})

	It("podman image prune dangling images", func() {
		podmanTest.BuildImage(pruneImage, "alpine_bash:latest", "true")
		podmanTest.BuildImage(pruneImage, "alpine_bash:latest", "true")
		none := podmanTest.Podman([]string{"images", "-a"})
		none.WaitWithDefaultTimeout()
		Expect(none).Should(Exit(0))
		hasNone, result := none.GrepString("<none>")
		Expect(result).To(HaveLen(2))
		Expect(hasNone).To(BeTrue())

		prune := podmanTest.Podman([]string{"image", "prune", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(Exit(0))

		after := podmanTest.Podman([]string{"images", "-a"})
		after.WaitWithDefaultTimeout()
		Expect(after).Should(Exit(0))
		hasNoneAfter, result := after.GrepString("<none>")
		Expect(hasNoneAfter).To(BeTrue())
		Expect(len(after.OutputToStringArray())).To(BeNumerically(">", 1))
		Expect(len(result)).To(BeNumerically(">", 0))
	})

	It("podman image prune unused images", func() {
		podmanTest.AddImageToRWStore(ALPINE)
		podmanTest.AddImageToRWStore(BB)

		images := podmanTest.Podman([]string{"images", "-a"})
		images.WaitWithDefaultTimeout()
		Expect(images).Should(Exit(0))

		prune := podmanTest.Podman([]string{"image", "prune", "-af"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(Exit(0))

		images = podmanTest.Podman([]string{"images", "-aq"})
		images.WaitWithDefaultTimeout()
		Expect(images).Should(Exit(0))
		// all images are unused, so they all should be deleted!
		Expect(images.OutputToStringArray()).To(HaveLen(len(CACHE_IMAGES)))
	})

	It("podman system image prune unused images", func() {
		podmanTest.AddImageToRWStore(ALPINE)
		podmanTest.BuildImage(pruneImage, "alpine_bash:latest", "true")
		prune := podmanTest.Podman([]string{"system", "prune", "-a", "--force"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(Exit(0))

		images := podmanTest.Podman([]string{"images", "-aq"})
		images.WaitWithDefaultTimeout()
		// all images are unused, so they all should be deleted!
		Expect(images.OutputToStringArray()).To(HaveLen(len(CACHE_IMAGES)))
	})

	It("podman system prune pods", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		podid1 := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podid1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"pod", "stop", podid1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		pods := podmanTest.Podman([]string{"pod", "ps"})
		pods.WaitWithDefaultTimeout()
		Expect(pods).Should(Exit(0))
		Expect(pods.OutputToStringArray()).To(HaveLen(3))

		prune := podmanTest.Podman([]string{"system", "prune", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(Exit(0))

		pods = podmanTest.Podman([]string{"pod", "ps"})
		pods.WaitWithDefaultTimeout()
		Expect(pods).Should(Exit(0))
		Expect(pods.OutputToStringArray()).To(HaveLen(2))
	})

	It("podman system prune - pod,container stopped", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		podid1 := session.OutputToString()

		// Start and stop a pod to get it in exited state.
		session = podmanTest.Podman([]string{"pod", "start", podid1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"pod", "stop", podid1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Create a container. This container should be pruned.
		create := podmanTest.Podman([]string{"create", "--name", "test", BB})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		prune := podmanTest.Podman([]string{"system", "prune", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(Exit(0))

		pods := podmanTest.Podman([]string{"pod", "ps"})
		pods.WaitWithDefaultTimeout()
		Expect(pods).Should(Exit(0))
		Expect(podmanTest.NumberOfPods()).To(Equal(0))

		Expect(podmanTest.NumberOfContainers()).To(Equal(0))
	})

	It("podman system prune with running, exited pod and volume prune set true", func() {
		// Start and stop a pod to get it in exited state.
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		podid1 := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podid1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"pod", "stop", podid1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Start a pod and leave it running
		session = podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		podid2 := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podid2})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

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
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"create", "-v", "myvol:/myvol", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(3))

		session = podmanTest.Podman([]string{"system", "prune", "--force", "--volumes"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Volumes should be pruned.
		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(BeEmpty())

		// One Pod should not be pruned as it was running
		Expect(podmanTest.NumberOfPods()).To(Equal(1))

		// Running pods infra container should not be pruned.
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		// Image should not be pruned and number should be same.
		images := podmanTest.Podman([]string{"images"})
		images.WaitWithDefaultTimeout()
		Expect(images.OutputToStringArray()).To(HaveLen(numberOfImages))
	})

	It("podman system prune - with dangling images true", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		podid1 := session.OutputToString()

		// Start and stop a pod to get it in exited state.
		session = podmanTest.Podman([]string{"pod", "start", podid1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"pod", "stop", podid1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Create a container. This container should be pruned.
		create := podmanTest.Podman([]string{"create", "--name", "test", BB})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		// Adding unused volume should not be pruned as volumes not set
		session = podmanTest.Podman([]string{"volume", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		prune := podmanTest.Podman([]string{"system", "prune", "-f", "-a"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(Exit(0))

		pods := podmanTest.Podman([]string{"pod", "ps"})
		pods.WaitWithDefaultTimeout()
		Expect(pods).Should(Exit(0))
		Expect(podmanTest.NumberOfPods()).To(Equal(0))

		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Volumes should not be pruned
		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(2))

		images := podmanTest.Podman([]string{"images", "-aq"})
		images.WaitWithDefaultTimeout()
		// all images are unused, so they all should be deleted!
		Expect(images.OutputToStringArray()).To(HaveLen(len(CACHE_IMAGES)))
	})

	It("podman system prune --volumes --filter", func() {
		session := podmanTest.Podman([]string{"volume", "create", "--label", "label1=value1", "myvol1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "create", "--label", "sharedlabel1=slv1", "myvol2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "create", "--label", "sharedlabel1=slv2", "myvol3"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "create", "--label", "sharedlabel1", "myvol4"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"create", "-v", "myvol5:/myvol5", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"create", "-v", "myvol6:/myvol6", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(7))

		session = podmanTest.Podman([]string{"system", "prune", "--force", "--volumes", "--filter", "label=label1=value1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(6))

		session = podmanTest.Podman([]string{"system", "prune", "--force", "--volumes", "--filter", "label=sharedlabel1=slv1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(5))

		session = podmanTest.Podman([]string{"system", "prune", "--force", "--volumes", "--filter", "label=sharedlabel1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(3))

		podmanTest.Cleanup()
	})
})
