package integration

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman create", func() {
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

	It("podman create container based on a local image", func() {
		session := podmanTest.Podman([]string{"create", "--name", "local_image_test", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		cid := session.OutputToString()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		check := podmanTest.Podman([]string{"inspect", "local_image_test"})
		check.WaitWithDefaultTimeout()
		data := check.InspectContainerToJSON()
		Expect(data[0].ID).To(ContainSubstring(cid))
	})

	It("podman create container based on a remote image", func() {
		session := podmanTest.Podman([]string{"create", BB_GLIBC, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
	})

	It("podman container create container based on a remote image", func() {
		session := podmanTest.Podman([]string{"container", "create", BB_GLIBC, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
	})

	It("podman create using short options", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
	})

	It("podman create using existing name", func() {
		session := podmanTest.Podman([]string{"create", "--name=foo", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session = podmanTest.Podman([]string{"create", "--name=foo", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman create adds annotation", func() {
		session := podmanTest.Podman([]string{"create", "--annotation", "HELLO=WORLD", "--name", "annotate_test", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		check := podmanTest.Podman([]string{"inspect", "annotate_test"})
		check.WaitWithDefaultTimeout()
		data := check.InspectContainerToJSON()
		value, ok := data[0].Config.Annotations["HELLO"]
		Expect(ok).To(BeTrue())
		Expect(value).To(Equal("WORLD"))
	})

	It("podman create --entrypoint command", func() {
		session := podmanTest.Podman([]string{"create", "--name", "entrypoint_test", "--entrypoint", "/bin/foobar", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		result := podmanTest.Podman([]string{"inspect", "entrypoint_test", "--format", "{{.Config.Entrypoint}}"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(Equal("/bin/foobar"))
	})

	It("podman create --entrypoint \"\"", func() {
		session := podmanTest.Podman([]string{"create", "--entrypoint", "", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		result := podmanTest.Podman([]string{"inspect", "-l", "--format", "{{.Config.Entrypoint}}"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(Equal(""))
	})

	It("podman create --entrypoint json", func() {
		jsonString := `[ "/bin/foo", "-c"]`
		session := podmanTest.Podman([]string{"create", "--name", "entrypoint_json", "--entrypoint", jsonString, ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		result := podmanTest.Podman([]string{"inspect", "entrypoint_json", "--format", "{{.Config.Entrypoint}}"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(Equal("/bin/foo -c"))
	})

	It("podman create --mount flag with multiple mounts", func() {
		Skip(v2remotefail)
		vol1 := filepath.Join(podmanTest.TempDir, "vol-test1")
		err := os.MkdirAll(vol1, 0755)
		Expect(err).To(BeNil())
		vol2 := filepath.Join(podmanTest.TempDir, "vol-test2")
		err = os.MkdirAll(vol2, 0755)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"create", "--name", "test", "--mount", "type=bind,src=" + vol1 + ",target=/myvol1,z", "--mount", "type=bind,src=" + vol2 + ",target=/myvol2,z", ALPINE, "touch", "/myvol2/foo.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"start", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"logs", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).ToNot(ContainSubstring("cannot touch"))
	})

	It("podman create with --mount flag", func() {
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("skip failing test on ppc64le")
		}
		Skip(v2remotefail)
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		os.Mkdir(mountPath, 0755)
		session := podmanTest.Podman([]string{"create", "--name", "test", "--mount", fmt.Sprintf("type=bind,src=%s,target=/create/test", mountPath), ALPINE, "grep", "/create/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"start", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"logs", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("/create/test rw"))

		session = podmanTest.Podman([]string{"create", "--name", "test_ro", "--mount", fmt.Sprintf("type=bind,src=%s,target=/create/test,ro", mountPath), ALPINE, "grep", "/create/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"start", "test_ro"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"logs", "test_ro"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("/create/test ro"))

		session = podmanTest.Podman([]string{"create", "--name", "test_shared", "--mount", fmt.Sprintf("type=bind,src=%s,target=/create/test,shared", mountPath), ALPINE, "grep", "/create/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"start", "test_shared"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"logs", "test_shared"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		found, matches := session.GrepString("/create/test")
		Expect(found).Should(BeTrue())
		Expect(matches[0]).To(ContainSubstring("rw"))
		Expect(matches[0]).To(ContainSubstring("shared"))

		mountPath = filepath.Join(podmanTest.TempDir, "scratchpad")
		os.Mkdir(mountPath, 0755)
		session = podmanTest.Podman([]string{"create", "--name", "test_tmpfs", "--mount", "type=tmpfs,target=/create/test", ALPINE, "grep", "/create/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"start", "test_tmpfs"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"logs", "test_tmpfs"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("/create/test rw,nosuid,nodev,noexec,relatime - tmpfs"))
	})

	It("podman create --pod automatically", func() {
		session := podmanTest.Podman([]string{"create", "--pod", "new:foobar", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		check := podmanTest.Podman([]string{"pod", "ps", "--no-trunc"})
		check.WaitWithDefaultTimeout()
		match, _ := check.GrepString("foobar")
		Expect(match).To(BeTrue())
	})

	It("podman run entrypoint and cmd test", func() {
		name := "test101"
		create := podmanTest.Podman([]string{"create", "--name", name, redis})
		create.WaitWithDefaultTimeout()
		Expect(create.ExitCode()).To(Equal(0))

		ctrJSON := podmanTest.InspectContainer(name)
		Expect(len(ctrJSON)).To(Equal(1))
		Expect(len(ctrJSON[0].Config.Cmd)).To(Equal(1))
		Expect(ctrJSON[0].Config.Cmd[0]).To(Equal("redis-server"))
		Expect(ctrJSON[0].Config.Entrypoint).To(Equal("docker-entrypoint.sh"))
	})

	It("podman create --pull", func() {
		session := podmanTest.PodmanNoCache([]string{"create", "--pull", "never", "--name=foo", "nginx"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.PodmanNoCache([]string{"create", "--pull", "always", "--name=foo", "nginx"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman create using image list by tag", func() {
		session := podmanTest.PodmanNoCache([]string{"create", "--pull=always", "--override-arch=arm64", "--name=foo", ALPINELISTTAG})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To((Equal(0)))
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.Image}}", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To((Equal(0)))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64ID))
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.ImageName}}", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To((Equal(0)))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTTAG))
	})

	It("podman create using image list by digest", func() {
		session := podmanTest.PodmanNoCache([]string{"create", "--pull=always", "--override-arch=arm64", "--name=foo", ALPINELISTDIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To((Equal(0)))
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.Image}}", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To((Equal(0)))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64ID))
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.ImageName}}", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To((Equal(0)))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTDIGEST))
	})

	It("podman create using image list instance by digest", func() {
		session := podmanTest.PodmanNoCache([]string{"create", "--pull=always", "--override-arch=arm64", "--name=foo", ALPINEARM64DIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To((Equal(0)))
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.Image}}", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To((Equal(0)))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64ID))
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.ImageName}}", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To((Equal(0)))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64DIGEST))
	})

	It("podman create using cross-arch image list instance by digest", func() {
		session := podmanTest.PodmanNoCache([]string{"create", "--pull=always", "--override-arch=arm64", "--name=foo", ALPINEARM64DIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To((Equal(0)))
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.Image}}", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To((Equal(0)))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64ID))
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.ImageName}}", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To((Equal(0)))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64DIGEST))
	})

	It("podman create --authfile with nonexist authfile", func() {
		SkipIfRemote()
		session := podmanTest.PodmanNoCache([]string{"create", "--authfile", "/tmp/nonexist", "--name=foo", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).To(Not(Equal(0)))
	})

	It("podman create with unset label", func() {
		// Alpine is assumed to have no labels here, which seems safe
		ctrName := "testctr"
		session := podmanTest.Podman([]string{"create", "--label", "TESTKEY1=", "--label", "TESTKEY2", "--name", ctrName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		data := inspect.InspectContainerToJSON()
		Expect(len(data)).To(Equal(1))
		Expect(len(data[0].Config.Labels)).To(Equal(2))
		_, ok1 := data[0].Config.Labels["TESTKEY1"]
		Expect(ok1).To(BeTrue())
		_, ok2 := data[0].Config.Labels["TESTKEY2"]
		Expect(ok2).To(BeTrue())
	})

	It("podman create with set label", func() {
		// Alpine is assumed to have no labels here, which seems safe
		ctrName := "testctr"
		session := podmanTest.Podman([]string{"create", "--label", "TESTKEY1=value1", "--label", "TESTKEY2=bar", "--name", ctrName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		data := inspect.InspectContainerToJSON()
		Expect(len(data)).To(Equal(1))
		Expect(len(data[0].Config.Labels)).To(Equal(2))
		val1, ok1 := data[0].Config.Labels["TESTKEY1"]
		Expect(ok1).To(BeTrue())
		Expect(val1).To(Equal("value1"))
		val2, ok2 := data[0].Config.Labels["TESTKEY2"]
		Expect(ok2).To(BeTrue())
		Expect(val2).To(Equal("bar"))
	})

	It("podman create with --restart=on-failure:5 parses correctly", func() {
		ctrName := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "--restart", "on-failure:5", "--name", ctrName, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		data := inspect.InspectContainerToJSON()
		Expect(len(data)).To(Equal(1))
		Expect(data[0].HostConfig.RestartPolicy.Name).To(Equal("on-failure"))
		Expect(data[0].HostConfig.RestartPolicy.MaximumRetryCount).To(Equal(uint(5)))
	})

	It("podman create with --restart-policy=always:5 fails", func() {
		session := podmanTest.Podman([]string{"create", "-t", "--restart", "always:5", ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
	})

	It("podman create with -m 1000000 sets swap to 2000000", func() {
		numMem := 1000000
		ctrName := "testCtr"
		session := podmanTest.Podman([]string{"create", "-t", "-m", fmt.Sprintf("%db", numMem), "--name", ctrName, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		data := inspect.InspectContainerToJSON()
		Expect(len(data)).To(Equal(1))
		Expect(data[0].HostConfig.MemorySwap).To(Equal(int64(2 * numMem)))
	})

	It("podman create --cpus 5 sets nanocpus", func() {
		numCpus := 5
		nanoCPUs := numCpus * 1000000000
		ctrName := "testCtr"
		session := podmanTest.Podman([]string{"create", "-t", "--cpus", fmt.Sprintf("%d", numCpus), "--name", ctrName, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		data := inspect.InspectContainerToJSON()
		Expect(len(data)).To(Equal(1))
		Expect(data[0].HostConfig.NanoCpus).To(Equal(int64(nanoCPUs)))
	})
})
