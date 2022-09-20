package integration

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/containers/podman/v4/pkg/rootless"
	. "github.com/containers/podman/v4/test/utils"
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
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)
	})

	// Returns the /proc/self/mountinfo line for a given mount point
	getMountInfo := func(volume string) []string {
		containerDir := strings.SplitN(volume, ":", 3)[1]
		session := podmanTest.Podman([]string{"run", "--rm", "-v", volume, ALPINE, "awk", fmt.Sprintf(`$5 == "%s" { print }`, containerDir), "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(containerDir), "mount point not found in /proc/self/mountinfo")
		return strings.Fields(session.OutputToString())
	}

	It("podman run with volume flag", func() {
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		err = os.Mkdir(mountPath, 0755)
		Expect(err).ToNot(HaveOccurred())
		vol := mountPath + ":" + dest

		// [5] is flags
		Expect(getMountInfo(vol)[5]).To(ContainSubstring("rw"))
		Expect(getMountInfo(vol + ":ro")[5]).To(ContainSubstring("ro"))

		mountinfo := getMountInfo(vol + ":shared")
		Expect(mountinfo[5]).To(ContainSubstring("rw"))
		Expect(mountinfo[6]).To(ContainSubstring("shared"))

		// Cached is ignored
		mountinfo = getMountInfo(vol + ":cached")
		Expect(mountinfo[5]).To(ContainSubstring("rw"))
		Expect(mountinfo).To(Not(ContainElement(ContainSubstring("cached"))))

		// Delegated is ignored
		mountinfo = getMountInfo(vol + ":delegated")
		Expect(mountinfo[5]).To(ContainSubstring("rw"))
		Expect(mountinfo).To(Not(ContainElement(ContainSubstring("delegated"))))
	})

	It("podman run with --mount flag", func() {
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("skip failing test on ppc64le")
		}
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		err = os.Mkdir(mountPath, 0755)
		Expect(err).ToNot(HaveOccurred())
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
		Expect(session.OutputToString()).To(ContainSubstring("rw"))
		Expect(session.OutputToString()).To(ContainSubstring("shared"))

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

		// test csv escaping
		session = podmanTest.Podman([]string{"run", "--rm", "--mount=type=tmpfs,tmpfs-size=512M,\"destination=/test,\"", ALPINE, "ls", "/test,"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

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
		err := os.Mkdir(mountPath, 0755)
		Expect(err).ToNot(HaveOccurred())
		session := podmanTest.Podman([]string{"run", "-v", mountPath + ":" + dest, "-v", "/tmp" + ":" + dest, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman run with conflict between image volume and user mount succeeds", func() {
		err = podmanTest.RestoreArtifact(REDIS_IMAGE)
		Expect(err).ToNot(HaveOccurred())
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		err := os.Mkdir(mountPath, 0755)
		Expect(err).To(BeNil())
		testFile := filepath.Join(mountPath, "test1")
		f, err := os.Create(testFile)
		Expect(err).To(BeNil(), "os.Create(testfile)")
		f.Close()
		Expect(err).To(BeNil())
		session := podmanTest.Podman([]string{"run", "-v", fmt.Sprintf("%s:/data", mountPath), REDIS_IMAGE, "ls", "/data/test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman run with mount flag and boolean options", func() {
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		err := os.Mkdir(mountPath, 0755)
		Expect(err).ToNot(HaveOccurred())
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
		err := os.Mkdir(mountPath, 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"run", "--rm", "-v", mountPath + ":" + dest + ":suid,dev,exec", ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		output := session.OutputToString()
		Expect(output).To(Not(ContainSubstring("noexec")))
		Expect(output).To(Not(ContainSubstring("nodev")))
		Expect(output).To(Not(ContainSubstring("nosuid")))

		session = podmanTest.Podman([]string{"run", "--rm", "--tmpfs", dest + ":suid,dev,exec", ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		output = session.OutputToString()
		Expect(output).To(Not(ContainSubstring("noexec")))
		Expect(output).To(Not(ContainSubstring("nodev")))
		Expect(output).To(Not(ContainSubstring("nosuid")))
	})

	// Container should start when workdir is overlay volume
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
		err := os.Mkdir(mountPath, 0755)
		Expect(err).ToNot(HaveOccurred())

		// Container should be able to start with custom overlay volume
		session := podmanTest.Podman([]string{"run", "--rm", "-v", mountPath + ":/data:O", "--workdir=/data", ALPINE, "echo", "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman support overlay on named volume", func() {
		SkipIfRemote("Overlay volumes only work locally")
		if os.Getenv("container") != "" {
			Skip("Overlay mounts not supported when running in a container")
		}
		if rootless.IsRootless() {
			if _, err := exec.LookPath("fuse-overlayfs"); err != nil {
				Skip("Fuse-Overlayfs required for rootless overlay mount test")
			}
		}
		session := podmanTest.Podman([]string{"volume", "create", "myvolume"})
		session.WaitWithDefaultTimeout()
		volName := session.OutputToString()
		Expect(session).Should(Exit(0))

		// create file on actual volume
		session = podmanTest.Podman([]string{"run", "--volume", volName + ":/data", ALPINE, "sh", "-c", "echo hello >> " + "/data/test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// create file on overlay volume
		session = podmanTest.Podman([]string{"run", "--volume", volName + ":/data:O", ALPINE, "sh", "-c", "echo hello >> " + "/data/overlay"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// volume should contain only `test` not `overlay`
		session = podmanTest.Podman([]string{"run", "--volume", volName + ":/data", ALPINE, "sh", "-c", "ls /data"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(Not(ContainSubstring("overlay")))
		Expect(session.OutputToString()).To(ContainSubstring("test"))

	})

	It("podman support overlay on named volume with custom upperdir and workdir", func() {
		SkipIfRemote("Overlay volumes only work locally")
		if os.Getenv("container") != "" {
			Skip("Overlay mounts not supported when running in a container")
		}
		if rootless.IsRootless() {
			if _, err := exec.LookPath("fuse-overlayfs"); err != nil {
				Skip("Fuse-Overlayfs required for rootless overlay mount test")
			}
		}

		// create persistent upperdir on host
		upperDir := filepath.Join(tempdir, "upper")
		err := os.Mkdir(upperDir, 0755)
		Expect(err).To(BeNil(), "mkdir "+upperDir)

		// create persistent workdir on host
		workDir := filepath.Join(tempdir, "work")
		err = os.Mkdir(workDir, 0755)
		Expect(err).To(BeNil(), "mkdir "+workDir)

		overlayOpts := fmt.Sprintf("upperdir=%s,workdir=%s", upperDir, workDir)

		session := podmanTest.Podman([]string{"volume", "create", "myvolume"})
		session.WaitWithDefaultTimeout()
		volName := session.OutputToString()
		Expect(session).Should(Exit(0))

		// create file on actual volume
		session = podmanTest.Podman([]string{"run", "--volume", volName + ":/data", ALPINE, "sh", "-c", "echo hello >> " + "/data/test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// create file on overlay volume
		session = podmanTest.Podman([]string{"run", "--volume", volName + ":/data:O," + overlayOpts, ALPINE, "sh", "-c", "echo hello >> " + "/data/overlay"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "--volume", volName + ":/data:O," + overlayOpts, ALPINE, "sh", "-c", "ls /data"})
		session.WaitWithDefaultTimeout()
		// must contain `overlay` file since it should be persistent on specified upper and workdir
		Expect(session.OutputToString()).To(ContainSubstring("overlay"))
		// this should be there since `test` was written on actual volume not on any overlay
		Expect(session.OutputToString()).To(ContainSubstring("test"))

		session = podmanTest.Podman([]string{"run", "--volume", volName + ":/data:O", ALPINE, "sh", "-c", "ls /data"})
		session.WaitWithDefaultTimeout()
		// must not contain `overlay` file which was on custom upper and workdir since we have not specified any upper or workdir
		Expect(session.OutputToString()).To(Not(ContainSubstring("overlay")))
		// this should be there since `test` was written on actual volume not on any overlay
		Expect(session.OutputToString()).To(ContainSubstring("test"))

	})

	It("podman support overlay volume with custom upperdir and workdir", func() {
		SkipIfRemote("Overlay volumes only work locally")
		if os.Getenv("container") != "" {
			Skip("Overlay mounts not supported when running in a container")
		}
		if rootless.IsRootless() {
			if _, err := exec.LookPath("fuse-overlayfs"); err != nil {
				Skip("Fuse-Overlayfs required for rootless overlay mount test")
			}
		}

		// Use bindsource instead of named volume
		bindSource := filepath.Join(tempdir, "bindsource")
		err := os.Mkdir(bindSource, 0755)
		Expect(err).To(BeNil(), "mkdir "+bindSource)

		// create persistent upperdir on host
		upperDir := filepath.Join(tempdir, "upper")
		err = os.Mkdir(upperDir, 0755)
		Expect(err).To(BeNil(), "mkdir "+upperDir)

		// create persistent workdir on host
		workDir := filepath.Join(tempdir, "work")
		err = os.Mkdir(workDir, 0755)
		Expect(err).To(BeNil(), "mkdir "+workDir)

		overlayOpts := fmt.Sprintf("upperdir=%s,workdir=%s", upperDir, workDir)

		// create file on overlay volume
		session := podmanTest.Podman([]string{"run", "--volume", bindSource + ":/data:O," + overlayOpts, ALPINE, "sh", "-c", "echo hello >> " + "/data/overlay"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "--volume", bindSource + ":/data:O," + overlayOpts, ALPINE, "sh", "-c", "ls /data"})
		session.WaitWithDefaultTimeout()
		// must contain `overlay` file since it should be persistent on specified upper and workdir
		Expect(session.OutputToString()).To(ContainSubstring("overlay"))

		session = podmanTest.Podman([]string{"run", "--volume", bindSource + ":/data:O", ALPINE, "sh", "-c", "ls /data"})
		session.WaitWithDefaultTimeout()
		// must not contain `overlay` file which was on custom upper and workdir since we have not specified any upper or workdir
		Expect(session.OutputToString()).To(Not(ContainSubstring("overlay")))

	})

	It("podman run with noexec can't exec", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "-v", "/bin:/hostbin:noexec", ALPINE, "/hostbin/ls", "/"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman run with tmpfs named volume mounts and unmounts", func() {
		SkipIfRootless("rootless podman mount requires you to be in a user namespace")
		SkipIfRemote("podman-remote does not support --volumes. This test could be simplified to be tested on Remote.")
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
		Expect(mountOut1).To(Not(ContainSubstring(volName)))

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
		Expect(mountOut2).To(ContainSubstring(volName))

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
		Expect(mountOut3).To(Not(ContainSubstring(volName)))
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

		copySession := podmanTest.Podman([]string{"run", "--rm", "-v", "testvol3:/etc/apk:copy", ALPINE, "stat", "-c", "%h", "/etc/apk/arch"})
		copySession.WaitWithDefaultTimeout()
		Expect(copySession).Should(Exit(0))

		noCopySession := podmanTest.Podman([]string{"run", "--rm", "-v", "testvol4:/etc/apk:nocopy", ALPINE, "stat", "-c", "%h", "/etc/apk/arch"})
		noCopySession.WaitWithDefaultTimeout()
		Expect(noCopySession).Should(Exit(1))
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
		Expect(arr).To(HaveLen(1))
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
		Expect(arr).To(HaveLen(1))
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
		Expect(arr).To(HaveLen(1))
		Expect(arr[0]).To(Equal(volName))

		remove := podmanTest.Podman([]string{"rm", "-v", ctrName})
		remove.WaitWithDefaultTimeout()
		Expect(remove).Should(Exit(0))

		list3 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list3.WaitWithDefaultTimeout()
		Expect(list3).Should(Exit(0))
		arr2 := list3.OutputToStringArray()
		Expect(arr2).To(HaveLen(1))
		Expect(arr2[0]).To(Equal(volName))
	})

	It("podman run image volume is not noexec", func() {
		session := podmanTest.Podman([]string{"run", "--rm", REDIS_IMAGE, "grep", "/data", "/proc/self/mountinfo"})
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
		Expect(outputArr).To(HaveLen(1))
		Expect(outputArr[0]).To(ContainSubstring(fileName))
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
		Expect(data).To(HaveLen(1))
		Expect(data[0].Mounts).To(HaveLen(1))
		Expect(data[0].Mounts[0]).To(HaveField("Source", "/tmp"))
		Expect(data[0].Mounts[0]).To(HaveField("Destination", "/test"))
	})

	It("podman run with overlay volume flag", func() {
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
		err := os.Mkdir(mountPath, 0755)
		Expect(err).ToNot(HaveOccurred())
		testFile := filepath.Join(mountPath, "test1")
		f, err := os.Create(testFile)
		Expect(err).To(BeNil(), "os.Create "+testFile)
		f.Close()

		// Make sure host directory gets mounted in to container as overlay
		volumeFlag := fmt.Sprintf("%s:/run/test:O", mountPath)
		Expect(getMountInfo(volumeFlag)[7]).To(Equal("overlay"))

		// Make sure host files show up in the container
		session := podmanTest.Podman([]string{"run", "--rm", "-v", volumeFlag, ALPINE, "ls", "/run/test/test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Test overlay mount when lowerdir is relative path.
		f, err = os.Create("hello")
		Expect(err).To(BeNil(), "os.Create")
		f.Close()
		session = podmanTest.Podman([]string{"run", "--rm", "-v", ".:/app:O", ALPINE, "ls", "/app"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(ContainSubstring("hello"))
		Expect(session).Should(Exit(0))

		// Make sure modifications in container do not show up on host
		session = podmanTest.Podman([]string{"run", "--rm", "-v", volumeFlag, ALPINE, "touch", "/run/test/container"})
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
		err := os.Mkdir(mountPath, 0755)
		Expect(err).ToNot(HaveOccurred())
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
		Expect(session.OutputToStringArray()).To(HaveLen(2))
	})

	It("podman run with --volume and U flag", func() {
		SkipIfRemote("Overlay volumes only work locally")

		u, err := user.Current()
		Expect(err).To(BeNil())
		name := u.Username
		if name == "root" {
			name = "containers"
		}

		content, err := os.ReadFile("/etc/subuid")
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
		err = os.Mkdir(mountPath, 0755)
		Expect(err).ToNot(HaveOccurred())
		vol := mountPath + ":" + dest + ":U"

		session := podmanTest.Podman([]string{"run", "--rm", "--user", "888:888", "-v", vol, ALPINE, "stat", "-c", "%u:%g", dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("888:888"))

		session = podmanTest.Podman([]string{"run", "--rm", "--user", "888:888", "--userns", "auto", "-v", vol, ALPINE, "stat", "-c", "%u:%g", dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("888:888"))

		vol += ",O"
		session = podmanTest.Podman([]string{"run", "--rm", "--user", "888:888", "--userns", "keep-id", "-v", vol, ALPINE, "stat", "-c", "%u:%g", dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("888:888"))
	})

	It("podman run with --mount and U flag", func() {
		u, err := user.Current()
		Expect(err).To(BeNil())
		name := u.Username
		if name == "root" {
			name = "containers"
		}

		content, err := os.ReadFile("/etc/subuid")
		if err != nil {
			Skip("cannot read /etc/subuid")
		}

		if !strings.Contains(string(content), name) {
			Skip("cannot find mappings for the current user")
		}

		mountPath := filepath.Join(podmanTest.TempDir, "foo")
		err = os.Mkdir(mountPath, 0755)
		Expect(err).ToNot(HaveOccurred())

		// false bind mount
		vol := "type=bind,src=" + mountPath + ",dst=" + dest + ",U=false"
		session := podmanTest.Podman([]string{"run", "--rm", "--user", "888:888", "--mount", vol, ALPINE, "stat", "-c", "%u:%g", dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).ShouldNot(Equal("888:888"))

		// invalid bind mount
		vol = "type=bind,src=" + mountPath + ",dst=" + dest + ",U=invalid"
		session = podmanTest.Podman([]string{"run", "--rm", "--user", "888:888", "--mount", vol, ALPINE, "stat", "-c", "%u:%g", dest})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		// true bind mount
		vol = "type=bind,src=" + mountPath + ",dst=" + dest + ",U=true"
		session = podmanTest.Podman([]string{"run", "--rm", "--user", "888:888", "--mount", vol, ALPINE, "stat", "-c", "%u:%g", dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).Should(Equal("888:888"))

		// tmpfs mount
		vol = "type=tmpfs," + "dst=" + dest + ",chown"
		session = podmanTest.Podman([]string{"run", "--rm", "--user", "888:888", "--mount", vol, ALPINE, "stat", "-c", "%u:%g", dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).Should(Equal("888:888"))

		// anonymous volume mount
		vol = "type=volume," + "dst=" + dest
		session = podmanTest.Podman([]string{"run", "--rm", "--user", "888:888", "--mount", vol, ALPINE, "stat", "-c", "%u:%g", dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).Should(Equal("888:888"))

		// named volume mount
		namedVolume := podmanTest.Podman([]string{"volume", "create", "foo"})
		namedVolume.WaitWithDefaultTimeout()
		Expect(namedVolume).Should(Exit(0))

		vol = "type=volume,src=foo,dst=" + dest + ",chown=true"
		session = podmanTest.Podman([]string{"run", "--rm", "--user", "888:888", "--mount", vol, ALPINE, "stat", "-c", "%u:%g", dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).Should(Equal("888:888"))
	})

	It("podman run with --mount and named volume with driver-opts", func() {
		// anonymous volume mount with driver opts
		vol := "type=volume,source=test_vol,dst=/test,volume-opt=type=tmpfs,volume-opt=device=tmpfs,volume-opt=o=nodev"
		session := podmanTest.Podman([]string{"run", "--rm", "--mount", vol, ALPINE, "echo", "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		inspectVol := podmanTest.Podman([]string{"volume", "inspect", "test_vol"})
		inspectVol.WaitWithDefaultTimeout()
		Expect(inspectVol).Should(Exit(0))
		Expect(inspectVol.OutputToString()).To(ContainSubstring("nodev"))
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
		Expect(test1.OutputToString()).To(ContainSubstring(testString))

		volName := "testvol2"
		vol := podmanTest.Podman([]string{"volume", "create", volName})
		vol.WaitWithDefaultTimeout()
		Expect(vol).Should(Exit(0))

		test2 := podmanTest.Podman([]string{"run", "-v", fmt.Sprintf("%s:/test", volName), imgName, "bash", "-c", "ls -al /test | grep -v root | grep -v total"})
		test2.WaitWithDefaultTimeout()
		Expect(test2).Should(Exit(0))
		Expect(test2.OutputToString()).To(ContainSubstring(testString))

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

	It("podman run with -v $SRC:/run does not create /run/.containerenv", func() {
		mountSrc := filepath.Join(podmanTest.TempDir, "vol-test1")
		err := os.MkdirAll(mountSrc, 0755)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"run", "-v", mountSrc + ":/run", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// the file should not have been created
		_, err = os.Stat(filepath.Join(mountSrc, ".containerenv"))
		Expect(err).To(Not(BeNil()))
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

	It("podman run -v with a relative dir", func() {
		mountPath := filepath.Join(podmanTest.TempDir, "vol")
		err = os.Mkdir(mountPath, 0755)
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			err := os.RemoveAll(mountPath)
			Expect(err).ToNot(HaveOccurred())
		}()

		f, err := os.CreateTemp(mountPath, "podman")
		Expect(err).ToNot(HaveOccurred())

		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())

		err = os.Chdir(mountPath)
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			err := os.Chdir(cwd)
			Expect(err).ToNot(HaveOccurred())
		}()

		run := podmanTest.Podman([]string{"run", "-it", "--security-opt", "label=disable", "-v", "./:" + dest, ALPINE, "ls", dest})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))
		Expect(run.OutputToString()).Should(ContainSubstring(strings.TrimLeft("/vol/", f.Name())))
	})
})
