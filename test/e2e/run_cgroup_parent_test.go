package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

const cgroupRoot = "/sys/fs/cgroup"

var _ = Describe("Podman run with --cgroup-parent", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		SkipIfRootlessCgroupsV1("cgroup parent is not supported in cgroups v1")
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

	Specify("valid --cgroup-parent using cgroupfs", func() {
		if !Containerized() {
			Skip("Must be containerized to run this test.")
		}
		cgroup := "/zzz"
		run := podmanTest.Podman([]string{"run", "--cgroupns=host", "--cgroup-parent", cgroup, fedoraMinimal, "cat", "/proc/self/cgroup"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))
		Expect(run.OutputToString()).To(ContainSubstring(cgroup))
	})

	Specify("no --cgroup-parent", func() {
		cgroup := "/libpod_parent"
		if !Containerized() && podmanTest.CgroupManager != "cgroupfs" {
			if isRootless() {
				cgroup = "/user.slice"
			} else {
				cgroup = "/machine.slice"
			}
		}
		run := podmanTest.Podman([]string{"run", "--cgroupns=host", fedoraMinimal, "cat", "/proc/self/cgroup"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))
		Expect(run.OutputToString()).To(ContainSubstring(cgroup))
	})

	Specify("always honor --cgroup-parent", func() {
		SkipIfCgroupV1("test not supported in cgroups v1")
		if Containerized() || podmanTest.CgroupManager == "cgroupfs" {
			Skip("Requires Systemd cgroup manager support")
		}
		if IsRemote() {
			Skip("Not supported for remote")
		}

		run := podmanTest.Podman([]string{"run", "-d", "--cgroupns=host", fedoraMinimal, "sleep", "100"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))
		cid := run.OutputToString()

		exec := podmanTest.Podman([]string{"exec", cid, "cat", "/proc/1/cgroup"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(Exit(0))

		containerCgroup := strings.TrimRight(strings.ReplaceAll(exec.OutputToString(), "0::", ""), "\n")

		// Move the container process to a sub cgroup
		content, err := os.ReadFile(filepath.Join(cgroupRoot, containerCgroup, "cgroup.procs"))
		Expect(err).ToNot(HaveOccurred())
		oldSubCgroupPath := filepath.Join(cgroupRoot, containerCgroup, "old-container")
		err = os.MkdirAll(oldSubCgroupPath, 0755)
		Expect(err).ToNot(HaveOccurred())
		err = os.WriteFile(filepath.Join(oldSubCgroupPath, "cgroup.procs"), content, 0644)
		Expect(err).ToNot(HaveOccurred())

		newCgroup := fmt.Sprintf("%s/new-container", containerCgroup)
		err = os.MkdirAll(filepath.Join(cgroupRoot, newCgroup), 0755)
		Expect(err).ToNot(HaveOccurred())

		run = podmanTest.Podman([]string{"--cgroup-manager=cgroupfs", "run", "--rm", "--cgroupns=host", fmt.Sprintf("--cgroup-parent=%s", newCgroup), fedoraMinimal, "cat", "/proc/self/cgroup"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))
		cgroupEffective := strings.TrimRight(strings.ReplaceAll(run.OutputToString(), "0::", ""), "\n")

		Expect(newCgroup).To(Equal(filepath.Dir(cgroupEffective)))
	})

	Specify("valid --cgroup-parent using slice", func() {
		if Containerized() || podmanTest.CgroupManager == "cgroupfs" {
			Skip("Requires Systemd cgroup manager support")
		}
		cgroup := "aaaa.slice"
		run := podmanTest.Podman([]string{"run", "--cgroupns=host", "--cgroup-parent", cgroup, fedoraMinimal, "cat", "/proc/1/cgroup"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))
		Expect(run.OutputToString()).To(ContainSubstring(cgroup))
	})
})
