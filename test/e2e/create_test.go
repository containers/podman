package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/containers/podman/v4/test/utils"
	"github.com/containers/storage/pkg/stringid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman create", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		Expect(err).ToNot(HaveOccurred())
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
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
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		check := podmanTest.Podman([]string{"inspect", "local_image_test"})
		check.WaitWithDefaultTimeout()
		data := check.InspectContainerToJSON()
		Expect(data[0].ID).To(ContainSubstring(cid))
	})

	It("podman create container based on a remote image", func() {
		session := podmanTest.Podman([]string{"create", BB_GLIBC, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
	})

	It("podman container create container based on a remote image", func() {
		containerCreate := podmanTest.Podman([]string{"container", "create", BB_GLIBC, "ls"})
		containerCreate.WaitWithDefaultTimeout()
		Expect(containerCreate).Should(Exit(0))

		lock := GetPortLock("5000")
		defer lock.Unlock()
		session := podmanTest.Podman([]string{"run", "-d", "--name", "registry", "-p", "5000:5000", REGISTRY_IMAGE, "/entrypoint.sh", "/etc/docker/registry/config.yml"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		if !WaitContainerReady(podmanTest, "registry", "listening on", 20, 1) {
			Skip("Cannot start docker registry.")
		}

		create := podmanTest.Podman([]string{"container", "create", "--tls-verify=false", ALPINE})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(3))
	})

	It("podman create using short options", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
	})

	It("podman create using existing name", func() {
		session := podmanTest.Podman([]string{"create", "--name=foo", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session = podmanTest.Podman([]string{"create", "--name=foo", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman create adds annotation", func() {
		session := podmanTest.Podman([]string{"create", "--annotation", "HELLO=WORLD", "--name", "annotate_test", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		check := podmanTest.Podman([]string{"inspect", "annotate_test"})
		check.WaitWithDefaultTimeout()
		data := check.InspectContainerToJSON()
		Expect(data[0].Config.Annotations).To(HaveKeyWithValue("HELLO", "WORLD"))
	})

	It("podman create --entrypoint command", func() {
		session := podmanTest.Podman([]string{"create", "--name", "entrypoint_test", "--entrypoint", "/bin/foobar", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		result := podmanTest.Podman([]string{"inspect", "entrypoint_test", "--format", "{{.Config.Entrypoint}}"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(Equal("/bin/foobar"))
	})

	It("podman create --entrypoint \"\"", func() {
		session := podmanTest.Podman([]string{"create", "--entrypoint", "", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		result := podmanTest.Podman([]string{"inspect", session.OutputToString(), "--format", "{{.Config.Entrypoint}}"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(Equal(""))
	})

	It("podman create --entrypoint json", func() {
		jsonString := `[ "/bin/foo", "-c"]`
		session := podmanTest.Podman([]string{"create", "--name", "entrypoint_json", "--entrypoint", jsonString, ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		result := podmanTest.Podman([]string{"inspect", "entrypoint_json", "--format", "{{.Config.Entrypoint}}"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(Equal("/bin/foo -c"))
	})

	It("podman create --mount flag with multiple mounts", func() {
		vol1 := filepath.Join(podmanTest.TempDir, "vol-test1")
		err := os.MkdirAll(vol1, 0755)
		Expect(err).ToNot(HaveOccurred())
		vol2 := filepath.Join(podmanTest.TempDir, "vol-test2")
		err = os.MkdirAll(vol2, 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"create", "--name", "test", "--mount", "type=bind,src=" + vol1 + ",target=/myvol1,z", "--mount", "type=bind,src=" + vol2 + ",target=/myvol2,z", ALPINE, "touch", "/myvol2/foo.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"start", "-a", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).ToNot(ContainSubstring("cannot touch"))
	})

	It("podman create with --mount flag", func() {
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("skip failing test on ppc64le")
		}

		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		err := os.Mkdir(mountPath, 0755)
		Expect(err).ToNot(HaveOccurred())
		session := podmanTest.Podman([]string{"create", "--name", "test", "--mount", fmt.Sprintf("type=bind,src=%s,target=/create/test", mountPath), ALPINE, "grep", "/create/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"start", "-a", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("/create/test rw"))

		session = podmanTest.Podman([]string{"create", "--name", "test_ro", "--mount", fmt.Sprintf("type=bind,src=%s,target=/create/test,ro", mountPath), ALPINE, "grep", "/create/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"start", "-a", "test_ro"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("/create/test ro"))

		session = podmanTest.Podman([]string{"create", "--name", "test_shared", "--mount", fmt.Sprintf("type=bind,src=%s,target=/create/test,shared", mountPath), ALPINE, "awk", `$5 == "/create/test" { print $6 " " $7}`, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"start", "-a", "test_shared"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("rw"))
		Expect(session.OutputToString()).To(ContainSubstring("shared"))

		mountPath = filepath.Join(podmanTest.TempDir, "scratchpad")
		err = os.Mkdir(mountPath, 0755)
		Expect(err).ToNot(HaveOccurred())
		session = podmanTest.Podman([]string{"create", "--name", "test_tmpfs", "--mount", "type=tmpfs,target=/create/test", ALPINE, "grep", "/create/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"start", "-a", "test_tmpfs"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("/create/test rw,nosuid,nodev,relatime - tmpfs"))
	})

	It("podman create --pod automatically", func() {
		session := podmanTest.Podman([]string{"create", "--pod", "new:foobar", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		check := podmanTest.Podman([]string{"pod", "ps", "--no-trunc"})
		check.WaitWithDefaultTimeout()
		Expect(check.OutputToString()).To(ContainSubstring("foobar"))
	})

	It("podman create --pod-id-file", func() {
		// First, make sure that --pod and --pod-id-file yield an error
		// if used together.
		session := podmanTest.Podman([]string{"create", "--pod", "foo", "--pod-id-file", "bar", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))

		tmpDir, err := os.MkdirTemp("", "")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(tmpDir)

		podName := "rudolph"
		ctrName := "prancer"
		podIDFile := tmpDir + "pod-id-file"

		// Now, let's create a pod with --pod-id-file.
		session = podmanTest.Podman([]string{"pod", "create", "--pod-id-file", podIDFile, "--name", podName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"pod", "inspect", podName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(BeValidJSON())
		podData := session.InspectPodToJSON()

		// Finally we can create a container with --pod-id-file and do
		// some checks to make sure it's working as expected.
		session = podmanTest.Podman([]string{"create", "--pod-id-file", podIDFile, "--name", ctrName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		ctrJSON := podmanTest.InspectContainer(ctrName)
		Expect(podData).To(HaveField("ID", ctrJSON[0].Pod)) // Make sure the container's pod matches the pod's ID
	})

	It("podman run entrypoint and cmd test", func() {
		name := "test101"
		create := podmanTest.Podman([]string{"create", "--name", name, REDIS_IMAGE})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		ctrJSON := podmanTest.InspectContainer(name)
		Expect(ctrJSON).To(HaveLen(1))
		Expect(ctrJSON[0].Config.Cmd).To(HaveLen(1))
		Expect(ctrJSON[0].Config.Cmd[0]).To(Equal("redis-server"))
		Expect(ctrJSON[0].Config).To(HaveField("Entrypoint", "docker-entrypoint.sh"))
	})

	It("podman create --pull", func() {
		session := podmanTest.Podman([]string{"create", "--pull", "never", "--name=foo", "testimage:00000000"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"create", "--pull", "always", "--name=foo", "testimage:00000000"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman create using image list by tag", func() {
		session := podmanTest.Podman([]string{"create", "--pull=always", "--arch=arm64", "--name=foo", ALPINELISTTAG})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.Image}}", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64ID))
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.ImageName}}", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTTAG))
	})

	It("podman create using image list by digest", func() {
		session := podmanTest.Podman([]string{"create", "--pull=always", "--arch=arm64", "--name=foo", ALPINELISTDIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.Image}}", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64ID))
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.ImageName}}", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTDIGEST))
	})

	It("podman create using image list instance by digest", func() {
		session := podmanTest.Podman([]string{"create", "--pull=always", "--arch=arm64", "--name=foo", ALPINEARM64DIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.Image}}", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64ID))
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.ImageName}}", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64DIGEST))
	})

	It("podman create using cross-arch image list instance by digest", func() {
		session := podmanTest.Podman([]string{"create", "--pull=always", "--arch=arm64", "--name=foo", ALPINEARM64DIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.Image}}", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64ID))
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.ImageName}}", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64DIGEST))
	})

	It("podman create --authfile with nonexistent authfile", func() {
		session := podmanTest.Podman([]string{"create", "--authfile", "/tmp/nonexistent", "--name=foo", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).To(Not(Equal(0)))
	})

	It("podman create --signature-policy", func() {
		session := podmanTest.Podman([]string{"create", "--pull=always", "--signature-policy", "/no/such/file", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"create", "--pull=always", "--signature-policy", "/etc/containers/policy.json", ALPINE})
		session.WaitWithDefaultTimeout()
		if IsRemote() {
			Expect(session).To(ExitWithError())
			Expect(session.ErrorToString()).To(ContainSubstring("unknown flag"))
		} else {
			Expect(session).Should(Exit(0))
		}
	})

	It("podman create with unset label", func() {
		// Alpine is assumed to have no labels here, which seems safe
		ctrName := "testctr"
		session := podmanTest.Podman([]string{"create", "--label", "TESTKEY1=", "--label", "TESTKEY2", "--name", ctrName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		data := inspect.InspectContainerToJSON()
		Expect(data).To(HaveLen(1), "len(InspectContainerToJSON)")
		Expect(data[0].Config.Labels).To(HaveLen(2))
		Expect(data[0].Config.Labels).To(HaveKey("TESTKEY1"))
		Expect(data[0].Config.Labels).To(HaveKey("TESTKEY2"))
	})

	It("podman create with set label", func() {
		// Alpine is assumed to have no labels here, which seems safe
		ctrName := "testctr"
		session := podmanTest.Podman([]string{"create", "--label", "TESTKEY1=value1", "--label", "TESTKEY2=bar", "--name", ctrName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		data := inspect.InspectContainerToJSON()
		Expect(data).To(HaveLen(1))
		Expect(data[0].Config.Labels).To(HaveLen(2))
		Expect(data[0].Config.Labels).To(HaveKeyWithValue("TESTKEY1", "value1"))
		Expect(data[0].Config.Labels).To(HaveKeyWithValue("TESTKEY2", "bar"))
	})

	It("podman create with --restart=on-failure:5 parses correctly", func() {
		ctrName := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "--restart", "on-failure:5", "--name", ctrName, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		data := inspect.InspectContainerToJSON()
		Expect(data).To(HaveLen(1))
		Expect(data[0].HostConfig.RestartPolicy).To(HaveField("Name", "on-failure"))
		Expect(data[0].HostConfig.RestartPolicy).To(HaveField("MaximumRetryCount", uint(5)))
	})

	It("podman create with --restart-policy=always:5 fails", func() {
		session := podmanTest.Podman([]string{"create", "-t", "--restart", "always:5", ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman create with --restart-policy unless-stopped", func() {
		ctrName := "testctr"
		unlessStopped := "unless-stopped"
		session := podmanTest.Podman([]string{"create", "-t", "--restart", unlessStopped, "--name", ctrName, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		data := inspect.InspectContainerToJSON()
		Expect(data).To(HaveLen(1))
		Expect(data[0].HostConfig.RestartPolicy).To(HaveField("Name", unlessStopped))
	})

	It("podman create with -m 1000000 sets swap to 2000000", func() {
		SkipIfRootlessCgroupsV1("Not supported for rootless + CgroupsV1")
		numMem := 1000000
		ctrName := "testCtr"
		session := podmanTest.Podman([]string{"create", "-t", "-m", fmt.Sprintf("%db", numMem), "--name", ctrName, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		data := inspect.InspectContainerToJSON()
		Expect(data).To(HaveLen(1))
		Expect(data[0].HostConfig).To(HaveField("MemorySwap", int64(2*numMem)))
	})

	It("podman create --cpus 5 sets nanocpus", func() {
		SkipIfRootlessCgroupsV1("Not supported for rootless + CgroupsV1")
		numCpus := 5
		nanoCPUs := numCpus * 1000000000
		ctrName := "testCtr"
		session := podmanTest.Podman([]string{"create", "-t", "--cpus", fmt.Sprintf("%d", numCpus), "--name", ctrName, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		data := inspect.InspectContainerToJSON()
		Expect(data).To(HaveLen(1))
		Expect(data[0].HostConfig).To(HaveField("NanoCpus", int64(nanoCPUs)))
	})

	It("podman create --replace", func() {
		// Make sure we error out with --name.
		session := podmanTest.Podman([]string{"create", "--replace", ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))

		// Create and replace 5 times in a row the "same" container.
		ctrName := "testCtr"
		for i := 0; i < 5; i++ {
			session = podmanTest.Podman([]string{"create", "--replace", "--name", ctrName, ALPINE, "/bin/sh"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
		}
	})

	It("podman create sets default stop signal 15", func() {
		ctrName := "testCtr"
		session := podmanTest.Podman([]string{"create", "--name", ctrName, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		data := inspect.InspectContainerToJSON()
		Expect(data).To(HaveLen(1))
		Expect(data[0].Config).To(HaveField("StopSignal", uint(15)))
	})

	It("podman create --tz", func() {
		session := podmanTest.Podman([]string{"create", "--tz", "foo", "--name", "bad", ALPINE, "date"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"create", "--tz", "America", "--name", "dir", ALPINE, "date"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"create", "--tz", "Pacific/Honolulu", "--name", "zone", ALPINE, "date"})
		session.WaitWithDefaultTimeout()
		inspect := podmanTest.Podman([]string{"inspect", "zone"})
		inspect.WaitWithDefaultTimeout()
		data := inspect.InspectContainerToJSON()
		Expect(data).To(HaveLen(1))
		Expect(data[0].Config).To(HaveField("Timezone", "Pacific/Honolulu"))

		session = podmanTest.Podman([]string{"create", "--tz", "local", "--name", "lcl", ALPINE, "date"})
		session.WaitWithDefaultTimeout()
		inspect = podmanTest.Podman([]string{"inspect", "lcl"})
		inspect.WaitWithDefaultTimeout()
		data = inspect.InspectContainerToJSON()
		Expect(data).To(HaveLen(1))
		Expect(data[0].Config).To(HaveField("Timezone", "local"))
	})

	It("podman create --umask", func() {
		if !strings.Contains(podmanTest.OCIRuntime, "crun") {
			Skip("Test only works on crun")
		}

		session := podmanTest.Podman([]string{"create", "--name", "default", ALPINE})
		session.WaitWithDefaultTimeout()
		inspect := podmanTest.Podman([]string{"inspect", "default"})
		inspect.WaitWithDefaultTimeout()
		data := inspect.InspectContainerToJSON()
		Expect(data).To(HaveLen(1))
		Expect(data[0].Config).To(HaveField("Umask", "0022"))

		session = podmanTest.Podman([]string{"create", "--umask", "0002", "--name", "umask", ALPINE})
		session.WaitWithDefaultTimeout()
		inspect = podmanTest.Podman([]string{"inspect", "umask"})
		inspect.WaitWithDefaultTimeout()
		data = inspect.InspectContainerToJSON()
		Expect(data).To(HaveLen(1))
		Expect(data[0].Config).To(HaveField("Umask", "0002"))

		session = podmanTest.Podman([]string{"create", "--umask", "0077", "--name", "fedora", fedoraMinimal})
		session.WaitWithDefaultTimeout()
		inspect = podmanTest.Podman([]string{"inspect", "fedora"})
		inspect.WaitWithDefaultTimeout()
		data = inspect.InspectContainerToJSON()
		Expect(data).To(HaveLen(1))
		Expect(data[0].Config).To(HaveField("Umask", "0077"))

		session = podmanTest.Podman([]string{"create", "--umask", "22", "--name", "umask-short", ALPINE})
		session.WaitWithDefaultTimeout()
		inspect = podmanTest.Podman([]string{"inspect", "umask-short"})
		inspect.WaitWithDefaultTimeout()
		data = inspect.InspectContainerToJSON()
		Expect(data).To(HaveLen(1))
		Expect(data[0].Config).To(HaveField("Umask", "0022"))

		session = podmanTest.Podman([]string{"create", "--umask", "9999", "--name", "bad", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		Expect(session.ErrorToString()).To(ContainSubstring("invalid umask"))
	})

	It("create container in pod with IP should fail", func() {
		SkipIfRootless("Setting IP not supported in rootless mode without network")
		name := "createwithstaticip"
		pod := podmanTest.RunTopContainerInPod("", "new:"+name)
		pod.WaitWithDefaultTimeout()
		Expect(pod).Should(Exit(0))

		session := podmanTest.Podman([]string{"create", "--pod", name, "--ip", "192.168.1.2", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError())
	})

	It("create container in pod with mac should fail", func() {
		SkipIfRootless("Setting MAC Address not supported in rootless mode without network")
		name := "createwithstaticmac"
		pod := podmanTest.RunTopContainerInPod("", "new:"+name)
		pod.WaitWithDefaultTimeout()
		Expect(pod).Should(Exit(0))

		session := podmanTest.Podman([]string{"create", "--pod", name, "--mac-address", "52:54:00:6d:2f:82", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError())
	})

	It("create container in pod with network should not fail", func() {
		name := "createwithnetwork"
		pod := podmanTest.RunTopContainerInPod("", "new:"+name)
		pod.WaitWithDefaultTimeout()
		Expect(pod).Should(Exit(0))

		netName := "pod" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		defer podmanTest.removeNetwork(netName)

		session = podmanTest.Podman([]string{"create", "--pod", name, "--network", netName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("create container in pod with ports should fail", func() {
		name := "createwithports"
		pod := podmanTest.RunTopContainerInPod("", "new:"+name)
		pod.WaitWithDefaultTimeout()
		Expect(pod).Should(Exit(0))

		session := podmanTest.Podman([]string{"create", "--pod", name, "-p", "8086:80", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError())
	})

	It("create container in pod publish ports should fail", func() {
		name := "createwithpublishports"
		pod := podmanTest.RunTopContainerInPod("", "new:"+name)
		pod.WaitWithDefaultTimeout()
		Expect(pod).Should(Exit(0))

		session := podmanTest.Podman([]string{"create", "--pod", name, "-P", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError())
	})

	It("create use local store image if input image contains a manifest list", func() {
		session := podmanTest.Podman([]string{"pull", BB})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "create", "mylist"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "add", "--all", "mylist", BB})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"create", "mylist"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman create -d should fail, can not detach create containers", func() {
		session := podmanTest.Podman([]string{"create", "-d", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		Expect(session.ErrorToString()).To(ContainSubstring("unknown shorthand flag"))

		session = podmanTest.Podman([]string{"create", "--detach", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		Expect(session.ErrorToString()).To(ContainSubstring("unknown flag"))

		session = podmanTest.Podman([]string{"create", "--detach-keys", "ctrl-x", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		Expect(session.ErrorToString()).To(ContainSubstring("unknown flag"))
	})

	It("podman create --platform", func() {
		session := podmanTest.Podman([]string{"create", "--platform=linux/bogus", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		expectedError := "no image found in manifest list for architecture bogus"
		Expect(session.ErrorToString()).To(ContainSubstring(expectedError))

		session = podmanTest.Podman([]string{"create", "--platform=linux/arm64", "--os", "windows", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		expectedError = "--platform option can not be specified with --arch or --os"
		Expect(session.ErrorToString()).To(ContainSubstring(expectedError))

		session = podmanTest.Podman([]string{"create", "-q", "--platform=linux/arm64", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		setup := podmanTest.Podman([]string{"container", "inspect", session.OutputToString()})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		data := setup.InspectContainerToJSON()
		setup = podmanTest.Podman([]string{"image", "inspect", data[0].Image})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		idata := setup.InspectImageJSON() // returns []inspect.ImageData
		Expect(idata).To(HaveLen(1))
		Expect(idata[0]).To(HaveField("Os", runtime.GOOS))
		Expect(idata[0]).To(HaveField("Architecture", "arm64"))
	})

	It("podman create --uid/gidmap --pod conflict test", func() {
		create := podmanTest.Podman([]string{"create", "--uidmap", "0:1000:1000", "--pod", "new:testing123", ALPINE})
		create.WaitWithDefaultTimeout()
		Expect(create).ShouldNot(Exit(0))
		Expect(create.ErrorToString()).To(ContainSubstring("cannot specify a new uid/gid map when entering a pod with an infra container"))

		create = podmanTest.Podman([]string{"create", "--gidmap", "0:1000:1000", "--pod", "new:testing1234", ALPINE})
		create.WaitWithDefaultTimeout()
		Expect(create).ShouldNot(Exit(0))
		Expect(create.ErrorToString()).To(ContainSubstring("cannot specify a new uid/gid map when entering a pod with an infra container"))

	})

	It("podman create --chrootdirs inspection test", func() {
		session := podmanTest.Podman([]string{"create", "--chrootdirs", "/var/local/qwerty", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		setup := podmanTest.Podman([]string{"container", "inspect", session.OutputToString()})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		data := setup.InspectContainerToJSON()
		Expect(data).To(HaveLen(1))
		Expect(data[0].Config.ChrootDirs).To(HaveLen(1))
		Expect(data[0].Config.ChrootDirs[0]).To(Equal("/var/local/qwerty"))
	})

	It("podman create --chrootdirs functionality test", func() {
		session := podmanTest.Podman([]string{"create", "-t", "--chrootdirs", "/var/local/qwerty", ALPINE, "/bin/cat"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		ctrID := session.OutputToString()

		setup := podmanTest.Podman([]string{"start", ctrID})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		setup = podmanTest.Podman([]string{"exec", ctrID, "cmp", "/etc/resolv.conf", "/var/local/qwerty/etc/resolv.conf"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))
	})

	It("create container with name subset of existing ID", func() {
		create1 := podmanTest.Podman([]string{"create", "-t", ALPINE, "top"})
		create1.WaitWithDefaultTimeout()
		Expect(create1).Should(Exit(0))
		ctr1ID := create1.OutputToString()

		ctr2Name := ctr1ID[:5]
		create2 := podmanTest.Podman([]string{"create", "-t", "--name", ctr2Name, ALPINE, "top"})
		create2.WaitWithDefaultTimeout()
		Expect(create2).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", "--format", "{{.Name}}", ctr2Name})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).Should(Equal(ctr2Name))
	})
})
