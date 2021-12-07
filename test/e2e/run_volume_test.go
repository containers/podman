package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/containers/podman/v3/pkg/rootless"
	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

// in-container mount point: using a path that is definitely not present
// on the host system might help to uncover some issues.
const dest = "/unique/path"

var _ = Describe("Podman run with volumes", func() {
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

	It("podman run with volume flag", func() {
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		os.Mkdir(mountPath, 0755)
		vol := mountPath + ":" + dest

		session := podmanTest.Podman([]string{"run", "--rm", "-v", vol, ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		found, matches := session.GrepString(dest)
		Expect(found).Should(BeTrue())
		Expect(matches[0]).To(ContainSubstring("rw"))

		session = podmanTest.Podman([]string{"run", "--rm", "-v", vol + ":ro", ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		found, matches = session.GrepString(dest)
		Expect(found).Should(BeTrue())
		Expect(matches[0]).To(ContainSubstring("ro"))

		session = podmanTest.Podman([]string{"run", "--rm", "-v", vol + ":shared", ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		found, matches = session.GrepString(dest)
		Expect(found).Should(BeTrue())
		Expect(matches[0]).To(ContainSubstring("rw"))
		Expect(matches[0]).To(ContainSubstring("shared"))

		// Cached is ignored
		session = podmanTest.Podman([]string{"run", "--rm", "-v", vol + ":cached", ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		found, matches = session.GrepString(dest)
		Expect(found).Should(BeTrue())
		Expect(matches[0]).To(ContainSubstring("rw"))
		Expect(matches[0]).To(Not(ContainSubstring("cached")))

		// Delegated is ignored
		session = podmanTest.Podman([]string{"run", "--rm", "-v", vol + ":delegated", ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		found, matches = session.GrepString(dest)
		Expect(found).Should(BeTrue())
		Expect(matches[0]).To(ContainSubstring("rw"))
		Expect(matches[0]).To(Not(ContainSubstring("delegated")))
	})

	It("podman run with --mount flag", func() {
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("skip failing test on ppc64le")
		}
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		os.Mkdir(mountPath, 0755)
		mount := "type=bind,src=" + mountPath + ",target=" + dest

		session := podmanTest.Podman([]string{"run", "--rm", "--mount", mount, ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(dest + " rw"))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", mount + ",ro", ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(dest + " ro"))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", mount + ",readonly", ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(dest + " ro"))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", mount + ",consistency=delegated,shared", ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		found, matches := session.GrepString(dest)
		Expect(found).Should(BeTrue())
		Expect(matches[0]).To(ContainSubstring("rw"))
		Expect(matches[0]).To(ContainSubstring("shared"))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", "type=tmpfs,target=" + dest, ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(dest + " rw,nosuid,nodev,relatime - tmpfs"))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", "type=tmpfs,target=/etc/ssl,tmpcopyup", ALPINE, "ls", "/etc/ssl"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("certs"))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", "type=tmpfs,target=/etc/ssl,tmpcopyup,notmpcopyup", ALPINE, "ls", "/etc/ssl"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", "type=bind,src=/tmp,target=/tmp,tmpcopyup", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", "type=bind,src=/tmp,target=/tmp,notmpcopyup", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", "type=tmpfs,target=/etc/ssl,notmpcopyup", ALPINE, "ls", "/etc/ssl"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Not(ContainSubstring("certs")))
	})

	It("podman run with conflicting volumes errors", func() {
		mountPath := filepath.Join(podmanTest.TmpDir, "secrets")
		os.Mkdir(mountPath, 0755)
		session := podmanTest.Podman([]string{"run", "-v", mountPath + ":" + dest, "-v", "/tmp" + ":" + dest, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman run with conflict between image volume and user mount succeeds", func() {
		podmanTest.RestoreArtifact(redis)
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		err := os.Mkdir(mountPath, 0755)
		Expect(err).To(BeNil())
		testFile := filepath.Join(mountPath, "test1")
		f, err := os.Create(testFile)
		f.Close()
		Expect(err).To(BeNil())
		session := podmanTest.Podman([]string{"run", "-v", fmt.Sprintf("%s:/data", mountPath), redis, "ls", "/data/test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman run with mount flag and boolean options", func() {
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		os.Mkdir(mountPath, 0755)
		mount := "type=bind,src=" + mountPath + ",target=" + dest

		session := podmanTest.Podman([]string{"run", "--rm", "--mount", mount + ",ro=false", ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(dest + " rw"))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", mount + ",ro=true", ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(dest + " ro"))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", mount + ",ro=true,rw=false", ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman run with volume flag and multiple named volumes", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "-v", "testvol1:/testvol1", "-v", "testvol2:/testvol2", ALPINE, "grep", "/testvol", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("/testvol1"))
		Expect(session.OutputToString()).To(ContainSubstring("/testvol2"))
	})

	It("podman run with volumes and suid/dev/exec options", func() {
		SkipIfRemote("podman-remote does not support --volumes")
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		os.Mkdir(mountPath, 0755)

		session := podmanTest.Podman([]string{"run", "--rm", "-v", mountPath + ":" + dest + ":suid,dev,exec", ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		found, matches := session.GrepString(dest)
		Expect(found).Should(BeTrue())
		Expect(matches[0]).To(Not(ContainSubstring("noexec")))
		Expect(matches[0]).To(Not(ContainSubstring("nodev")))
		Expect(matches[0]).To(Not(ContainSubstring("nosuid")))

		session = podmanTest.Podman([]string{"run", "--rm", "--tmpfs", dest + ":suid,dev,exec", ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		found, matches = session.GrepString(dest)
		Expect(found).Should(BeTrue())
		Expect(matches[0]).To(Not(ContainSubstring("noexec")))
		Expect(matches[0]).To(Not(ContainSubstring("nodev")))
		Expect(matches[0]).To(Not(ContainSubstring("nosuid")))
	})

	// Container should start when workdir is overlayed volume
	It("podman run with volume mounted as overlay and used as workdir", func() {
		SkipIfRemote("Overlay volumes only work locally")
		if os.Getenv("container") != "" {
			Skip("Overlay mounts not supported when running in a container")
		}
		if rootless.IsRootless() {
			if _, err := exec.LookPath("fuse-overlayfs"); err != nil {
				Skip("Fuse-Overlayfs required for rootless overlay mount test")
			}
		}
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		os.Mkdir(mountPath, 0755)

		//Container should be able to start with custom overlayed volume
		session := podmanTest.Podman([]string{"run", "--rm", "-v", mountPath + ":/data:O", "--workdir=/data", ALPINE, "echo", "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman run with noexec can't exec", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "-v", "/bin:/hostbin:noexec", ALPINE, "/hostbin/ls", "/"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman run with tmpfs named volume mounts and unmounts", func() {
		SkipIfRootless("FIXME:  rootless podman mount requires you to be in a user namespace")
		SkipIfRemote("podman-remote does not support --volumes this test could be simplified to be tested on Remote.")
		volName := "testvol"
		mkVolume := podmanTest.Podman([]string{"volume", "create", "--opt", "type=tmpfs", "--opt", "device=tmpfs", "--opt", "o=nodev", "testvol"})
		mkVolume.WaitWithDefaultTimeout()
		Expect(mkVolume).Should(Exit(0))

		// Volume not mounted on create
		mountCmd1, err := Start(exec.Command("mount"), GinkgoWriter, GinkgoWriter)
		Expect(err).To(BeNil())
		mountCmd1.Wait(90)
		Expect(mountCmd1).Should(Exit(0))
		os.Stdout.Sync()
		os.Stderr.Sync()
		mountOut1 := strings.Join(strings.Fields(string(mountCmd1.Out.Contents())), " ")
		fmt.Printf("Output: %s", mountOut1)
		Expect(strings.Contains(mountOut1, volName)).To(BeFalse())

		ctrName := "testctr"
		podmanSession := podmanTest.Podman([]string{"run", "-d", "--name", ctrName, "-v", fmt.Sprintf("%s:/testvol", volName), ALPINE, "top"})
		podmanSession.WaitWithDefaultTimeout()
		Expect(podmanSession).Should(Exit(0))

		// Volume now mounted as container is running
		mountCmd2, err := Start(exec.Command("mount"), GinkgoWriter, GinkgoWriter)
		Expect(err).To(BeNil())
		mountCmd2.Wait(90)
		Expect(mountCmd2).Should(Exit(0))
		os.Stdout.Sync()
		os.Stderr.Sync()
		mountOut2 := strings.Join(strings.Fields(string(mountCmd2.Out.Contents())), " ")
		fmt.Printf("Output: %s", mountOut2)
		Expect(strings.Contains(mountOut2, volName)).To(BeTrue())

		// Stop the container to unmount
		podmanStopSession := podmanTest.Podman([]string{"stop", "--time", "0", ctrName})
		podmanStopSession.WaitWithDefaultTimeout()
		Expect(podmanStopSession).Should(Exit(0))

		// We have to force cleanup so the unmount happens
		podmanCleanupSession := podmanTest.Podman([]string{"container", "cleanup", ctrName})
		podmanCleanupSession.WaitWithDefaultTimeout()
		Expect(podmanCleanupSession).Should(Exit(0))

		// Ensure volume is unmounted
		mountCmd3, err := Start(exec.Command("mount"), GinkgoWriter, GinkgoWriter)
		Expect(err).To(BeNil())
		mountCmd3.Wait(90)
		Expect(mountCmd3).Should(Exit(0))
		os.Stdout.Sync()
		os.Stderr.Sync()
		mountOut3 := strings.Join(strings.Fields(string(mountCmd3.Out.Contents())), " ")
		fmt.Printf("Output: %s", mountOut3)
		Expect(strings.Contains(mountOut3, volName)).To(BeFalse())
	})

	It("podman named volume copyup", func() {
		baselineSession := podmanTest.Podman([]string{"run", "--rm", "-t", "-i", ALPINE, "ls", "/etc/apk/"})
		baselineSession.WaitWithDefaultTimeout()
		Expect(baselineSession).Should(Exit(0))
		baselineOutput := baselineSession.OutputToString()

		inlineVolumeSession := podmanTest.Podman([]string{"run", "--rm", "-t", "-i", "-v", "testvol1:/etc/apk", ALPINE, "ls", "/etc/apk/"})
		inlineVolumeSession.WaitWithDefaultTimeout()
		Expect(inlineVolumeSession).Should(Exit(0))
		Expect(inlineVolumeSession.OutputToString()).To(Equal(baselineOutput))

		makeVolumeSession := podmanTest.Podman([]string{"volume", "create", "testvol2"})
		makeVolumeSession.WaitWithDefaultTimeout()
		Expect(makeVolumeSession).Should(Exit(0))

		separateVolumeSession := podmanTest.Podman([]string{"run", "--rm", "-t", "-i", "-v", "testvol2:/etc/apk", ALPINE, "ls", "/etc/apk/"})
		separateVolumeSession.WaitWithDefaultTimeout()
		Expect(separateVolumeSession).Should(Exit(0))
		Expect(separateVolumeSession.OutputToString()).To(Equal(baselineOutput))
	})

	It("podman named volume copyup symlink", func() {
		imgName := "testimg"
		dockerfile := fmt.Sprintf(`FROM %s
RUN touch /testfile
RUN sh -c "cd /etc/apk && ln -s ../../testfile"`, ALPINE)
		podmanTest.BuildImage(dockerfile, imgName, "false")

		baselineSession := podmanTest.Podman([]string{"run", "--rm", "-t", "-i", imgName, "ls", "/etc/apk/"})
		baselineSession.WaitWithDefaultTimeout()
		Expect(baselineSession).Should(Exit(0))
		baselineOutput := baselineSession.OutputToString()

		outputSession := podmanTest.Podman([]string{"run", "-t", "-i", "-v", "/etc/apk/", imgName, "ls", "/etc/apk/"})
		outputSession.WaitWithDefaultTimeout()
		Expect(outputSession).Should(Exit(0))
		Expect(outputSession.OutputToString()).To(Equal(baselineOutput))
	})

	It("podman named volume copyup empty directory", func() {
		baselineSession := podmanTest.Podman([]string{"run", "--rm", "-t", "-i", ALPINE, "ls", "/srv"})
		baselineSession.WaitWithDefaultTimeout()
		Expect(baselineSession).Should(Exit(0))
		baselineOutput := baselineSession.OutputToString()

		outputSession := podmanTest.Podman([]string{"run", "-t", "-i", "-v", "/srv", ALPINE, "ls", "/srv"})
		outputSession.WaitWithDefaultTimeout()
		Expect(outputSession).Should(Exit(0))
		Expect(outputSession.OutputToString()).To(Equal(baselineOutput))
	})

	It("podman named volume copyup of /var", func() {
		baselineSession := podmanTest.Podman([]string{"run", "--rm", "-t", "-i", fedoraMinimal, "ls", "/var"})
		baselineSession.WaitWithDefaultTimeout()
		Expect(baselineSession).Should(Exit(0))
		baselineOutput := baselineSession.OutputToString()

		outputSession := podmanTest.Podman([]string{"run", "-t", "-i", "-v", "/var", fedoraMinimal, "ls", "/var"})
		outputSession.WaitWithDefaultTimeout()
		Expect(outputSession).Should(Exit(0))
		Expect(outputSession.OutputToString()).To(Equal(baselineOutput))
	})

	It("podman read-only tmpfs conflict with volume", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "-t", "-i", "--read-only", "-v", "tmp_volume:" + dest, ALPINE, "touch", dest + "/a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session2 := podmanTest.Podman([]string{"run", "--rm", "-t", "-i", "--read-only", "--tmpfs", dest, ALPINE, "touch", dest + "/a"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))
	})

	It("podman run with anonymous volume", func() {
		list1 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list1.WaitWithDefaultTimeout()
		Expect(list1).Should(Exit(0))
		Expect(list1.OutputToString()).To(Equal(""))

		session := podmanTest.Podman([]string{"create", "-v", "/test", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		list2 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list2.WaitWithDefaultTimeout()
		Expect(list2).Should(Exit(0))
		arr := list2.OutputToStringArray()
		Expect(len(arr)).To(Equal(1))
		Expect(arr[0]).To(Not(Equal("")))
	})

	It("podman rm -v removes anonymous volume", func() {
		list1 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list1.WaitWithDefaultTimeout()
		Expect(list1).Should(Exit(0))
		Expect(list1.OutputToString()).To(Equal(""))

		ctrName := "testctr"
		session := podmanTest.Podman([]string{"create", "--name", ctrName, "-v", "/test", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		list2 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list2.WaitWithDefaultTimeout()
		Expect(list2).Should(Exit(0))
		arr := list2.OutputToStringArray()
		Expect(len(arr)).To(Equal(1))
		Expect(arr[0]).To(Not(Equal("")))

		remove := podmanTest.Podman([]string{"rm", "-v", ctrName})
		remove.WaitWithDefaultTimeout()
		Expect(remove).Should(Exit(0))

		list3 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list3.WaitWithDefaultTimeout()
		Expect(list3).Should(Exit(0))
		Expect(list3.OutputToString()).To(Equal(""))
	})

	It("podman rm -v retains named volume", func() {
		list1 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list1.WaitWithDefaultTimeout()
		Expect(list1).Should(Exit(0))
		Expect(list1.OutputToString()).To(Equal(""))

		ctrName := "testctr"
		volName := "testvol"
		session := podmanTest.Podman([]string{"create", "--name", ctrName, "-v", fmt.Sprintf("%s:/test", volName), ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		list2 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list2.WaitWithDefaultTimeout()
		Expect(list2).Should(Exit(0))
		arr := list2.OutputToStringArray()
		Expect(len(arr)).To(Equal(1))
		Expect(arr[0]).To(Equal(volName))

		remove := podmanTest.Podman([]string{"rm", "-v", ctrName})
		remove.WaitWithDefaultTimeout()
		Expect(remove).Should(Exit(0))

		list3 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list3.WaitWithDefaultTimeout()
		Expect(list3).Should(Exit(0))
		arr2 := list3.OutputToStringArray()
		Expect(len(arr2)).To(Equal(1))
		Expect(arr2[0]).To(Equal(volName))
	})

	It("podman run image volume is not noexec", func() {
		session := podmanTest.Podman([]string{"run", "--rm", redis, "grep", "/data", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Not(ContainSubstring("noexec")))
	})

	It("podman mount with invalid option fails", func() {
		volName := "testVol"
		volCreate := podmanTest.Podman([]string{"volume", "create", "--opt", "type=tmpfs", "--opt", "device=tmpfs", "--opt", "o=invalid", volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(Exit(0))

		volMount := podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:/tmp", volName), ALPINE, "ls"})
		volMount.WaitWithDefaultTimeout()
		Expect(volMount).To(ExitWithError())
	})

	It("Podman fix for CVE-2020-1726", func() {
		volName := "testVol"
		volCreate := podmanTest.Podman([]string{"volume", "create", volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(Exit(0))

		volPath := podmanTest.Podman([]string{"volume", "inspect", "--format", "{{.Mountpoint}}", volName})
		volPath.WaitWithDefaultTimeout()
		Expect(volPath).Should(Exit(0))
		path := volPath.OutputToString()

		fileName := "thisIsATestFile"
		file, err := os.Create(filepath.Join(path, fileName))
		Expect(err).To(BeNil())
		defer file.Close()

		runLs := podmanTest.Podman([]string{"run", "-t", "-i", "--rm", "-v", fmt.Sprintf("%v:/etc/ssl", volName), ALPINE, "ls", "-1", "/etc/ssl"})
		runLs.WaitWithDefaultTimeout()
		Expect(runLs).Should(Exit(0))
		outputArr := runLs.OutputToStringArray()
		Expect(len(outputArr)).To(Equal(1))
		Expect(strings.Contains(outputArr[0], fileName)).To(BeTrue())
	})

	It("Podman mount over image volume with trailing /", func() {
		image := "podman-volume-test:trailing"
		dockerfile := fmt.Sprintf(`FROM %s
VOLUME /test/`, ALPINE)
		podmanTest.BuildImage(dockerfile, image, "false")

		ctrName := "testCtr"
		create := podmanTest.Podman([]string{"create", "-v", "/tmp:/test", "--name", ctrName, image, "ls"})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		data := podmanTest.InspectContainer(ctrName)
		Expect(len(data)).To(Equal(1))
		Expect(len(data[0].Mounts)).To(Equal(1))
		Expect(data[0].Mounts[0].Source).To(Equal("/tmp"))
		Expect(data[0].Mounts[0].Destination).To(Equal("/test"))
	})

	It("podman run with overlay volume flag", func() {
		SkipIfRemote("Overlay volumes only work locally")
		if os.Getenv("container") != "" {
			Skip("Overlay mounts not supported when running in a container")
		}
		if rootless.IsRootless() {
			if _, err := exec.LookPath("fuse_overlay"); err != nil {
				Skip("Fuse-Overlayfs required for rootless overlay mount test")
			}
		}
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		os.Mkdir(mountPath, 0755)
		testFile := filepath.Join(mountPath, "test1")
		f, err := os.Create(testFile)
		f.Close()

		// Make sure host directory gets mounted in to container as overlay
		session := podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:/run/test:O", mountPath), ALPINE, "grep", "/run/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		found, matches := session.GrepString("/run/test")
		Expect(found).Should(BeTrue())
		Expect(matches[0]).To(ContainSubstring("overlay"))

		// Make sure host files show up in the container
		session = podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:/run/test:O", mountPath), ALPINE, "ls", "/run/test/test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Make sure modifications in container do not show up on host
		session = podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:/run/test:O", mountPath), ALPINE, "touch", "/run/test/container"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		_, err = os.Stat(filepath.Join(mountPath, "container"))
		Expect(err).To(Not(BeNil()))

		// Make sure modifications in container disappear when container is stopped
		session = podmanTest.Podman([]string{"create", "-v", fmt.Sprintf("%s:/run/test:O", mountPath), ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"start", "-l"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"exec", "-l", "touch", "/run/test/container"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"exec", "-l", "ls", "/run/test/container"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"stop", "-l"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"start", "-l"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"exec", "-l", "ls", "/run/test/container"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("overlay volume conflicts with named volume and mounts", func() {
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		os.Mkdir(mountPath, 0755)
		testFile := filepath.Join(mountPath, "test1")
		f, err := os.Create(testFile)
		Expect(err).To(BeNil())
		f.Close()
		mountSrc := filepath.Join(podmanTest.TempDir, "vol-test1")
		err = os.MkdirAll(mountSrc, 0755)
		Expect(err).To(BeNil())
		mountDest := "/run/test"
		volName := "myvol"

		session := podmanTest.Podman([]string{"volume", "create", volName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// overlay and named volume destinations conflict
		session = podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:%s:O", mountPath, mountDest), "-v", fmt.Sprintf("%s:%s", volName, mountDest), ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		// overlay and bind mount destinations conflict
		session = podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:%s:O", mountPath, mountDest), "--mount", fmt.Sprintf("type=bind,src=%s,target=%s", mountSrc, mountDest), ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		// overlay and tmpfs mount destinations conflict
		session = podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:%s:O", mountPath, mountDest), "--mount", fmt.Sprintf("type=tmpfs,target=%s", mountDest), ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("same volume in multiple places does not deadlock", func() {
		volName := "testVol1"
		session := podmanTest.Podman([]string{"run", "-t", "-i", "-v", fmt.Sprintf("%s:/test1", volName), "-v", fmt.Sprintf("%s:/test2", volName), "--rm", ALPINE, "sh", "-c", "mount | grep /test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(len(session.OutputToStringArray())).To(Equal(2))
	})

	It("podman run with U volume flag", func() {
		SkipIfRemote("Overlay volumes only work locally")

		u, err := user.Current()
		Expect(err).To(BeNil())
		name := u.Username
		if name == "root" {
			name = "containers"
		}

		content, err := ioutil.ReadFile("/etc/subuid")
		if err != nil {
			Skip("cannot read /etc/subuid")
		}
		if !strings.Contains(string(content), name) {
			Skip("cannot find mappings for the current user")
		}

		if os.Getenv("container") != "" {
			Skip("Overlay mounts not supported when running in a container")
		}
		if rootless.IsRootless() {
			if _, err := exec.LookPath("fuse_overlay"); err != nil {
				Skip("Fuse-Overlayfs required for rootless overlay mount test")
			}
		}

		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		os.Mkdir(mountPath, 0755)
		vol := mountPath + ":" + dest + ":U"

		session := podmanTest.Podman([]string{"run", "--rm", "--user", "888:888", "-v", vol, ALPINE, "stat", "-c", "%u:%g", dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		found, _ := session.GrepString("888:888")
		Expect(found).Should(BeTrue())

		session = podmanTest.Podman([]string{"run", "--rm", "--user", "888:888", "--userns", "auto", "-v", vol, ALPINE, "stat", "-c", "%u:%g", dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		found, _ = session.GrepString("888:888")
		Expect(found).Should(BeTrue())

		vol = vol + ",O"
		session = podmanTest.Podman([]string{"run", "--rm", "--user", "888:888", "--userns", "keep-id", "-v", vol, ALPINE, "stat", "-c", "%u:%g", dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		found, _ = session.GrepString("888:888")
		Expect(found).Should(BeTrue())
	})

	It("volume permissions after run", func() {
		imgName := "testimg"
		dockerfile := fmt.Sprintf(`FROM %s
RUN useradd -m testuser -u 1005
USER testuser`, fedoraMinimal)
		podmanTest.BuildImage(dockerfile, imgName, "false")

		testString := "testuser testuser"

		test1 := podmanTest.Podman([]string{"run", "-v", "testvol1:/test", imgName, "bash", "-c", "ls -al /test | grep -v root | grep -v total"})
		test1.WaitWithDefaultTimeout()
		Expect(test1).Should(Exit(0))
		Expect(strings.Contains(test1.OutputToString(), testString)).To(BeTrue())

		volName := "testvol2"
		vol := podmanTest.Podman([]string{"volume", "create", volName})
		vol.WaitWithDefaultTimeout()
		Expect(vol).Should(Exit(0))

		test2 := podmanTest.Podman([]string{"run", "-v", fmt.Sprintf("%s:/test", volName), imgName, "bash", "-c", "ls -al /test | grep -v root | grep -v total"})
		test2.WaitWithDefaultTimeout()
		Expect(test2).Should(Exit(0))
		Expect(strings.Contains(test2.OutputToString(), testString)).To(BeTrue())

	})

	It("podman run with named volume check if we honor permission of target dir", func() {
		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "stat", "-c", "%a %Y", "/var/tmp"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		perms := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--rm", "-v", "test:/var/tmp", ALPINE, "stat", "-c", "%a %Y", "/var/tmp"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal(perms))
	})

	It("podman volume with uid and gid works", func() {
		volName := "testVol"
		volCreate := podmanTest.Podman([]string{"volume", "create", "--opt", "o=uid=1000", volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(Exit(0))

		volMount := podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:/test", volName), ALPINE, "stat", "-c", "%u", "/test"})
		volMount.WaitWithDefaultTimeout()
		Expect(volMount).Should(Exit(0))
		Expect(volMount.OutputToString()).To(Equal("1000"))

		volName = "testVol2"
		volCreate = podmanTest.Podman([]string{"volume", "create", "--opt", "o=gid=1000", volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(Exit(0))

		volMount = podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:/test", volName), ALPINE, "stat", "-c", "%g", "/test"})
		volMount.WaitWithDefaultTimeout()
		Expect(volMount).Should(Exit(0))
		Expect(volMount.OutputToString()).To(Equal("1000"))

		volName = "testVol3"
		volCreate = podmanTest.Podman([]string{"volume", "create", "--opt", "o=uid=1000,gid=1000", volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(Exit(0))

		volMount = podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:/test", volName), ALPINE, "stat", "-c", "%u:%g", "/test"})
		volMount.WaitWithDefaultTimeout()
		Expect(volMount).Should(Exit(0))
		Expect(volMount.OutputToString()).To(Equal("1000:1000"))
	})
})
