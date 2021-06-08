package integration

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman logs", func() {
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

	for _, log := range []string{"k8s-file", "journald", "json-file"} {

		It("all lines: "+log, func() {
			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(Exit(0))
			cid := logc.OutputToString()

			results := podmanTest.Podman([]string{"logs", cid})
			results.WaitWithDefaultTimeout()
			Expect(results).To(Exit(0))
			Expect(len(results.OutputToStringArray())).To(Equal(3))
			Expect(results.OutputToString()).To(Equal("podman podman podman"))
		})

		It("tail two lines: "+log, func() {
			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(Exit(0))
			cid := logc.OutputToString()

			results := podmanTest.Podman([]string{"logs", "--tail", "2", cid})
			results.WaitWithDefaultTimeout()
			Expect(results).To(Exit(0))
			Expect(len(results.OutputToStringArray())).To(Equal(2))
		})

		It("tail zero lines: "+log, func() {
			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(Exit(0))
			cid := logc.OutputToString()

			results := podmanTest.Podman([]string{"logs", "--tail", "0", cid})
			results.WaitWithDefaultTimeout()
			Expect(results).To(Exit(0))
			Expect(len(results.OutputToStringArray())).To(Equal(0))
		})

		It("tail 99 lines: "+log, func() {
			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(Exit(0))
			cid := logc.OutputToString()

			results := podmanTest.Podman([]string{"logs", "--tail", "99", cid})
			results.WaitWithDefaultTimeout()
			Expect(results).To(Exit(0))
			Expect(len(results.OutputToStringArray())).To(Equal(3))
		})

		It("tail 800 lines: "+log, func() {
			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "-dt", ALPINE, "sh", "-c", "i=1; while [ \"$i\" -ne 1000 ]; do echo \"line $i\"; i=$((i + 1)); done"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(Exit(0))
			cid := logc.OutputToString()

			results := podmanTest.Podman([]string{"logs", "--tail", "800", cid})
			results.WaitWithDefaultTimeout()
			Expect(results).To(Exit(0))
			Expect(len(results.OutputToStringArray())).To(Equal(800))
		})

		It("tail 2 lines with timestamps: "+log, func() {
			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(Exit(0))
			cid := logc.OutputToString()

			results := podmanTest.Podman([]string{"logs", "--tail", "2", "-t", cid})
			results.WaitWithDefaultTimeout()
			Expect(results).To(Exit(0))
			Expect(len(results.OutputToStringArray())).To(Equal(2))
		})

		It("since time 2017-08-07: "+log, func() {
			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(Exit(0))
			cid := logc.OutputToString()

			results := podmanTest.Podman([]string{"logs", "--since", "2017-08-07T10:10:09.056611202-04:00", cid})
			results.WaitWithDefaultTimeout()
			Expect(results).To(Exit(0))
			Expect(len(results.OutputToStringArray())).To(Equal(3))
		})

		It("since duration 10m: "+log, func() {
			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(Exit(0))
			cid := logc.OutputToString()

			results := podmanTest.Podman([]string{"logs", "--since", "10m", cid})
			results.WaitWithDefaultTimeout()
			Expect(results).To(Exit(0))
			Expect(len(results.OutputToStringArray())).To(Equal(3))
		})

		It("latest and container name should fail: "+log, func() {
			results := podmanTest.Podman([]string{"logs", "-l", "foobar"})
			results.WaitWithDefaultTimeout()
			Expect(results).To(ExitWithError())
		})

		It("two containers showing short container IDs: "+log, func() {
			SkipIfRemote("FIXME: podman-remote logs does not support showing two containers at the same time")
			log1 := podmanTest.Podman([]string{"run", "--log-driver", log, "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
			log1.WaitWithDefaultTimeout()
			Expect(log1.ExitCode()).To(Equal(0))
			cid1 := log1.OutputToString()

			log2 := podmanTest.Podman([]string{"run", "--log-driver", log, "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
			log2.WaitWithDefaultTimeout()
			Expect(log2.ExitCode()).To(Equal(0))
			cid2 := log2.OutputToString()

			results := podmanTest.Podman([]string{"logs", cid1, cid2})
			results.WaitWithDefaultTimeout()
			Expect(results).Should(Exit(0))

			output := results.OutputToStringArray()
			Expect(len(output)).To(Equal(6))
			Expect(strings.Contains(output[0], cid1[:12]) || strings.Contains(output[0], cid2[:12])).To(BeTrue())
		})

		It("podman logs on a created container should result in 0 exit code: "+log, func() {
			session := podmanTest.Podman([]string{"create", "--log-driver", log, "-t", "--name", "log", ALPINE})
			session.WaitWithDefaultTimeout()
			Expect(session).To(Exit(0))

			results := podmanTest.Podman([]string{"logs", "log"})
			results.WaitWithDefaultTimeout()
			Expect(results).To(Exit(0))
		})

		It("streaming output: "+log, func() {
			containerName := "logs-f"

			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "--name", containerName, "-dt", ALPINE, "sh", "-c", "echo podman-1; sleep 1; echo podman-2"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(Exit(0))

			results := podmanTest.Podman([]string{"logs", "-f", containerName})
			results.WaitWithDefaultTimeout()
			Expect(results).To(Exit(0))

			Expect(results.OutputToString()).To(ContainSubstring("podman-1"))
			Expect(results.OutputToString()).To(ContainSubstring("podman-2"))

			// Container should now be terminatING or terminatED, but we
			// have no guarantee of which: 'logs -f' does not necessarily
			// wait for cleanup. Run 'inspect' and accept either state.
			inspect := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.State.Status}}", containerName})
			inspect.WaitWithDefaultTimeout()
			if inspect.ExitCode() == 0 {
				Expect(inspect.OutputToString()).To(Equal("exited"))
				// TODO: add 2-second wait loop to confirm cleanup
			} else {
				Expect(inspect.ErrorToString()).To(ContainSubstring("no such container"))
			}

			results = podmanTest.Podman([]string{"rm", "-f", containerName})
			results.WaitWithDefaultTimeout()
			Expect(results).To(Exit(0))
		})

		It("follow output stopped container: "+log, func() {
			containerName := "logs-f"

			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "--name", containerName, "-d", ALPINE, "true"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(Exit(0))

			results := podmanTest.Podman([]string{"logs", "-f", containerName})
			results.WaitWithDefaultTimeout()
			Expect(results).To(Exit(0))
		})

		It("using container with container log-size: "+log, func() {
			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "--log-opt=max-size=10k", "-d", ALPINE, "sh", "-c", "echo podman podman podman"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(Exit(0))
			cid := logc.OutputToString()

			wait := podmanTest.Podman([]string{"wait", cid})
			wait.WaitWithDefaultTimeout()
			Expect(wait).To(Exit(0))

			inspect := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.HostConfig.LogConfig.Size}}", cid})
			inspect.WaitWithDefaultTimeout()
			Expect(inspect).To(Exit(0))
			Expect(inspect.OutputToString()).To(Equal("10kB"))

			results := podmanTest.Podman([]string{"logs", cid})
			results.WaitWithDefaultTimeout()
			Expect(results).To(Exit(0))
			Expect(results.OutputToString()).To(Equal("podman podman podman"))
		})

		It("Make sure logs match expected length: "+log, func() {
			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "-t", "--name", "test", ALPINE, "sh", "-c", "echo 1; echo 2"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(Exit(0))

			wait := podmanTest.Podman([]string{"wait", "test"})
			wait.WaitWithDefaultTimeout()
			Expect(wait).To(Exit(0))

			results := podmanTest.Podman([]string{"logs", "test"})
			results.WaitWithDefaultTimeout()
			Expect(results).To(Exit(0))
			outlines := results.OutputToStringArray()
			Expect(len(outlines)).To(Equal(2))
			Expect(outlines[0]).To(Equal("1\r"))
			Expect(outlines[1]).To(Equal("2\r"))
		})

		It("podman logs test stdout and stderr: "+log, func() {
			cname := "log-test"
			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "--name", cname, ALPINE, "sh", "-c", "echo stdout; echo stderr >&2"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(Exit(0))

			wait := podmanTest.Podman([]string{"wait", cname})
			wait.WaitWithDefaultTimeout()
			Expect(wait).To(Exit(0))

			results := podmanTest.Podman([]string{"logs", cname})
			results.WaitWithDefaultTimeout()
			Expect(results).To(Exit(0))
			Expect(results.OutputToString()).To(Equal("stdout"))
			Expect(results.ErrorToString()).To(Equal("stderr"))
		})
	}

	It("using journald for container with container tag", func() {
		SkipIfInContainer("journalctl inside a container doesn't work correctly")
		logc := podmanTest.Podman([]string{"run", "--log-driver", "journald", "--log-opt=tag={{.ImageName}}", "-d", ALPINE, "sh", "-c", "echo podman; sleep 0.1; echo podman; sleep 0.1; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(Exit(0))
		cid := logc.OutputToString()

		wait := podmanTest.Podman([]string{"wait", cid})
		wait.WaitWithDefaultTimeout()
		Expect(wait).To(Exit(0))

		cmd := exec.Command("journalctl", "--no-pager", "-o", "json", "--output-fields=CONTAINER_TAG", fmt.Sprintf("CONTAINER_ID_FULL=%s", cid))
		out, err := cmd.CombinedOutput()
		Expect(err).To(BeNil())
		Expect(string(out)).To(ContainSubstring("alpine"))
	})

	It("using journald container name", func() {
		SkipIfInContainer("journalctl inside a container doesn't work correctly")
		containerName := "inside-journal"
		logc := podmanTest.Podman([]string{"run", "--log-driver", "journald", "-d", "--name", containerName, ALPINE, "sh", "-c", "echo podman; sleep 0.1; echo podman; sleep 0.1; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(Exit(0))
		cid := logc.OutputToString()

		wait := podmanTest.Podman([]string{"wait", cid})
		wait.WaitWithDefaultTimeout()
		Expect(wait).To(Exit(0))

		cmd := exec.Command("journalctl", "--no-pager", "-o", "json", "--output-fields=CONTAINER_NAME", fmt.Sprintf("CONTAINER_ID_FULL=%s", cid))
		out, err := cmd.CombinedOutput()
		Expect(err).To(BeNil())
		Expect(string(out)).To(ContainSubstring(containerName))
	})

	It("podman logs with log-driver=none errors", func() {
		ctrName := "logsctr"
		logc := podmanTest.Podman([]string{"run", "--name", ctrName, "-d", "--log-driver", "none", ALPINE, "top"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(Exit(0))

		logs := podmanTest.Podman([]string{"logs", "-f", ctrName})
		logs.WaitWithDefaultTimeout()
		Expect(logs).To(Not(Exit(0)))
	})
})
