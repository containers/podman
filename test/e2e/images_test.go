package integration

import (
	"fmt"
	"os"
	"sort"
	"strings"

	. "github.com/containers/libpod/test/utils"
	"github.com/docker/go-units"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman images", func() {
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
	It("podman images", func() {
		session := podmanTest.Podman([]string{"images"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 2))
		Expect(session.LineInOuputStartsWith("docker.io/library/alpine")).To(BeTrue())
		Expect(session.LineInOuputStartsWith("docker.io/library/busybox")).To(BeTrue())
	})

	It("podman images with no images prints header", func() {
		rmi := podmanTest.PodmanNoCache([]string{"rmi", "-a"})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(Exit(0))

		session := podmanTest.PodmanNoCache([]string{"images"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(len(session.OutputToStringArray())).To(Equal(1))
		Expect(session.LineInOutputContains("REPOSITORY")).To(BeTrue())
	})

	It("podman image List", func() {
		session := podmanTest.Podman([]string{"image", "list"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 2))
		Expect(session.LineInOuputStartsWith("docker.io/library/alpine")).To(BeTrue())
		Expect(session.LineInOuputStartsWith("docker.io/library/busybox")).To(BeTrue())
	})

	It("podman images with multiple tags", func() {
		// tag "docker.io/library/alpine:latest" to "foo:{a,b,c}"
		podmanTest.RestoreAllArtifacts()
		session := podmanTest.PodmanNoCache([]string{"tag", ALPINE, "foo:a", "foo:b", "foo:c"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		// tag "foo:c" to "bar:{a,b}"
		session = podmanTest.PodmanNoCache([]string{"tag", "foo:c", "bar:a", "bar:b"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		// check all previous and the newly tagged images
		session = podmanTest.PodmanNoCache([]string{"images"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session.LineInOutputContainsTag("docker.io/library/alpine", "latest")
		session.LineInOutputContainsTag("docker.io/library/busybox", "glibc")
		session.LineInOutputContainsTag("foo", "a")
		session.LineInOutputContainsTag("foo", "b")
		session.LineInOutputContainsTag("foo", "c")
		session.LineInOutputContainsTag("bar", "a")
		session.LineInOutputContainsTag("bar", "b")
		session = podmanTest.PodmanNoCache([]string{"images", "-qn"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically("==", 3))
	})

	It("podman images with digests", func() {
		session := podmanTest.Podman([]string{"images", "--digests"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 2))
		Expect(session.LineInOuputStartsWith("docker.io/library/alpine")).To(BeTrue())
		Expect(session.LineInOuputStartsWith("docker.io/library/busybox")).To(BeTrue())
	})

	It("podman empty images list in JSON format", func() {
		session := podmanTest.Podman([]string{"images", "--format=json", "not-existing-image"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.IsJSONOutputValid()).To(BeTrue())
	})

	It("podman images in JSON format", func() {
		session := podmanTest.Podman([]string{"images", "--format=json"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.IsJSONOutputValid()).To(BeTrue())
	})

	It("podman images in GO template format", func() {
		formatStr := "{{.ID}}\t{{.Created}}\t{{.CreatedAt}}\t{{.CreatedSince}}\t{{.CreatedTime}}"
		session := podmanTest.Podman([]string{"images", fmt.Sprintf("--format=%s", formatStr)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman images with short options", func() {
		session := podmanTest.Podman([]string{"images", "-qn"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 1))
	})

	It("podman images filter by image name", func() {
		podmanTest.RestoreAllArtifacts()
		session := podmanTest.PodmanNoCache([]string{"images", "-q", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(len(session.OutputToStringArray())).To(Equal(1))

		session = podmanTest.PodmanNoCache([]string{"tag", ALPINE, "foo:a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.PodmanNoCache([]string{"tag", BB, "foo:b"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.PodmanNoCache([]string{"images", "-q", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(len(session.OutputToStringArray())).To(Equal(2))
	})

	It("podman images filter reference", func() {
		SkipIfRemote()
		podmanTest.RestoreAllArtifacts()
		result := podmanTest.PodmanNoCache([]string{"images", "-q", "-f", "reference=docker.io*"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(len(result.OutputToStringArray())).To(Equal(2))

		retapline := podmanTest.PodmanNoCache([]string{"images", "-f", "reference=a*pine"})
		retapline.WaitWithDefaultTimeout()
		Expect(retapline).Should(Exit(0))
		Expect(len(retapline.OutputToStringArray())).To(Equal(3))
		Expect(retapline.LineInOutputContains("alpine")).To(BeTrue())

		retapline = podmanTest.PodmanNoCache([]string{"images", "-f", "reference=alpine"})
		retapline.WaitWithDefaultTimeout()
		Expect(retapline).Should(Exit(0))
		Expect(len(retapline.OutputToStringArray())).To(Equal(3))
		Expect(retapline.LineInOutputContains("alpine")).To(BeTrue())

		retnone := podmanTest.PodmanNoCache([]string{"images", "-q", "-f", "reference=bogus"})
		retnone.WaitWithDefaultTimeout()
		Expect(retnone).Should(Exit(0))
		Expect(len(retnone.OutputToStringArray())).To(Equal(0))
	})

	It("podman images filter before image", func() {
		SkipIfRemote()
		dockerfile := `FROM docker.io/library/alpine:latest
RUN apk update && apk add man
`
		podmanTest.BuildImage(dockerfile, "foobar.com/before:latest", "false")
		result := podmanTest.Podman([]string{"images", "-q", "-f", "before=foobar.com/before:latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(len(result.OutputToStringArray()) >= 1).To(BeTrue())
	})

	It("podman images filter after image", func() {
		SkipIfRemote()
		podmanTest.RestoreAllArtifacts()
		rmi := podmanTest.PodmanNoCache([]string{"rmi", "busybox"})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(Exit(0))

		dockerfile := `FROM docker.io/library/alpine:latest
`
		podmanTest.BuildImage(dockerfile, "foobar.com/before:latest", "false")
		result := podmanTest.PodmanNoCache([]string{"images", "-q", "-f", "after=docker.io/library/alpine:latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(len(result.OutputToStringArray())).To(Equal(0))
	})

	It("podman image list filter after image", func() {
		SkipIfRemote()
		podmanTest.RestoreAllArtifacts()
		rmi := podmanTest.PodmanNoCache([]string{"image", "rm", "busybox"})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(Exit(0))

		dockerfile := `FROM docker.io/library/alpine:latest
`
		podmanTest.BuildImage(dockerfile, "foobar.com/before:latest", "false")
		result := podmanTest.PodmanNoCache([]string{"image", "list", "-q", "-f", "after=docker.io/library/alpine:latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(len(result.OutputToStringArray())).To(Equal(0))
	})

	It("podman images filter dangling", func() {
		SkipIfRemote()
		dockerfile := `FROM docker.io/library/alpine:latest
`
		podmanTest.BuildImage(dockerfile, "foobar.com/before:latest", "false")
		podmanTest.BuildImage(dockerfile, "foobar.com/before:latest", "false")
		result := podmanTest.Podman([]string{"images", "-q", "-f", "dangling=true"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(len(result.OutputToStringArray())).To(Equal(0))
	})

	It("podman check for image with sha256: prefix", func() {
		if podmanTest.RemoteTest {
			Skip("Does not work on remote client")
		}
		session := podmanTest.Podman([]string{"inspect", "--format=json", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.IsJSONOutputValid()).To(BeTrue())
		imageData := session.InspectImageJSON()

		result := podmanTest.Podman([]string{"images", "sha256:" + imageData[0].ID})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
	})

	It("podman check for image with sha256: prefix", func() {
		if podmanTest.RemoteTest {
			Skip("Does not work on remote client")
		}
		session := podmanTest.Podman([]string{"image", "inspect", "--format=json", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.IsJSONOutputValid()).To(BeTrue())
		imageData := session.InspectImageJSON()

		result := podmanTest.Podman([]string{"image", "ls", fmt.Sprintf("sha256:%s", imageData[0].ID)})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
	})

	It("podman images sort by values", func() {
		sortValueTest := func(value string, result int, format string) []string {
			f := fmt.Sprintf("{{.%s}}", format)
			session := podmanTest.Podman([]string{"images", "--sort", value, "--format", f})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(result))

			return session.OutputToStringArray()
		}

		sortedArr := sortValueTest("created", 0, "CreatedAt")
		Expect(sort.SliceIsSorted(sortedArr, func(i, j int) bool { return sortedArr[i] > sortedArr[j] })).To(BeTrue())

		sortedArr = sortValueTest("id", 0, "ID")
		Expect(sort.SliceIsSorted(sortedArr, func(i, j int) bool { return sortedArr[i] < sortedArr[j] })).To(BeTrue())

		sortedArr = sortValueTest("repository", 0, "Repository")
		Expect(sort.SliceIsSorted(sortedArr, func(i, j int) bool { return sortedArr[i] < sortedArr[j] })).To(BeTrue())

		sortedArr = sortValueTest("size", 0, "Size")
		Expect(sort.SliceIsSorted(sortedArr, func(i, j int) bool {
			size1, _ := units.FromHumanSize(sortedArr[i])
			size2, _ := units.FromHumanSize(sortedArr[j])
			return size1 < size2
		})).To(BeTrue())
		sortedArr = sortValueTest("tag", 0, "Tag")
		Expect(sort.SliceIsSorted(sortedArr,
			func(i, j int) bool { return sortedArr[i] < sortedArr[j] })).
			To(BeTrue())

		sortValueTest("badvalue", 125, "Tag")
		sortValueTest("id", 125, "badvalue")
	})

	It("podman images --all flag", func() {
		SkipIfRemote()
		podmanTest.RestoreAllArtifacts()
		dockerfile := `FROM docker.io/library/alpine:latest
RUN mkdir hello
RUN touch test.txt
ENV foo=bar
`
		podmanTest.BuildImage(dockerfile, "test", "true")
		session := podmanTest.PodmanNoCache([]string{"images"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(len(session.OutputToStringArray())).To(Equal(5))

		session2 := podmanTest.PodmanNoCache([]string{"images", "--all"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))
		Expect(len(session2.OutputToStringArray())).To(Equal(7))
	})

	It("podman images filter by label", func() {
		SkipIfRemote()
		dockerfile := `FROM docker.io/library/alpine:latest
LABEL version="1.0"
LABEL "com.example.vendor"="Example Vendor"
`
		podmanTest.BuildImage(dockerfile, "test", "true")
		session := podmanTest.Podman([]string{"images", "-f", "label=version=1.0"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(len(session.OutputToStringArray())).To(Equal(2))
	})

	It("podman with images with no layers", func() {
		SkipIfRemote()
		dockerfile := strings.Join([]string{
			`FROM scratch`,
			`LABEL org.opencontainers.image.authors="<somefolks@example.org>"`,
			`LABEL org.opencontainers.image.created=2019-06-11T19:03:37Z`,
			`LABEL org.opencontainers.image.description="This is a test image"`,
			`LABEL org.opencontainers.image.title=test`,
			`LABEL org.opencontainers.image.vendor="Example.org"`,
			`LABEL org.opencontainers.image.version=1`,
		}, "\n")
		podmanTest.BuildImage(dockerfile, "foo", "true")

		session := podmanTest.Podman([]string{"images", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		output := session.OutputToString()
		Expect(output).To(Not(MatchRegexp("<missing>")))
		Expect(output).To(Not(MatchRegexp("error")))

		session = podmanTest.Podman([]string{"image", "tree", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		output = session.OutputToString()
		Expect(output).To(MatchRegexp("No Image Layers"))

		session = podmanTest.Podman([]string{"history", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		output = session.OutputToString()
		Expect(output).To(Not(MatchRegexp("error")))

		session = podmanTest.Podman([]string{"history", "--quiet", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(len(session.OutputToStringArray())).To(Equal(6))

		session = podmanTest.Podman([]string{"image", "list", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		output = session.OutputToString()
		Expect(output).To(Not(MatchRegexp("<missing>")))
		Expect(output).To(Not(MatchRegexp("error")))

		session = podmanTest.Podman([]string{"image", "list"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		output = session.OutputToString()
		Expect(output).To(Not(MatchRegexp("<missing>")))
		Expect(output).To(Not(MatchRegexp("error")))

		session = podmanTest.Podman([]string{"inspect", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		output = session.OutputToString()
		Expect(output).To(Not(MatchRegexp("<missing>")))
		Expect(output).To(Not(MatchRegexp("error")))

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RootFS.Layers}}", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		output = session.OutputToString()
		Expect(output).To(Equal("[]"))
	})

	It("podman images --filter readonly", func() {
		SkipIfRemote()
		dockerfile := `FROM docker.io/library/alpine:latest
`
		podmanTest.BuildImage(dockerfile, "foobar.com/before:latest", "false")
		result := podmanTest.Podman([]string{"images", "-f", "readonly=true"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		result1 := podmanTest.Podman([]string{"images", "--filter", "readonly=false"})
		result1.WaitWithDefaultTimeout()
		Expect(result1).Should(Exit(0))
		Expect(result.OutputToStringArray()).To(Not(Equal(result1.OutputToStringArray())))
	})

})
