package integration

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	. "github.com/containers/libpod/test/utils"
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

	It("all lines", func() {
		logc := podmanTest.Podman([]string{"run", "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(Exit(0))

		cid := logc.OutputToString()
		results := podmanTest.Podman([]string{"logs", cid})
		results.WaitWithDefaultTimeout()
		Expect(results).To(Exit(0))
		Expect(len(results.OutputToStringArray())).To(Equal(3))
	})

	It("tail two lines", func() {
		logc := podmanTest.Podman([]string{"run", "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(Exit(0))
		cid := logc.OutputToString()

		results := podmanTest.Podman([]string{"logs", "--tail", "2", cid})
		results.WaitWithDefaultTimeout()
		Expect(results).To(Exit(0))
		Expect(len(results.OutputToStringArray())).To(Equal(2))
	})

	It("tail zero lines", func() {
		logc := podmanTest.Podman([]string{"run", "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(Exit(0))
		cid := logc.OutputToString()

		results := podmanTest.Podman([]string{"logs", "--tail", "0", cid})
		results.WaitWithDefaultTimeout()
		Expect(results).To(Exit(0))
		Expect(len(results.OutputToStringArray())).To(Equal(0))
	})

	It("tail 99 lines", func() {
		logc := podmanTest.Podman([]string{"run", "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(Exit(0))
		cid := logc.OutputToString()

		results := podmanTest.Podman([]string{"logs", "--tail", "99", cid})
		results.WaitWithDefaultTimeout()
		Expect(results).To(Exit(0))
		Expect(len(results.OutputToStringArray())).To(Equal(3))
	})

	It("tail 2 lines with timestamps", func() {
		logc := podmanTest.Podman([]string{"run", "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(Exit(0))
		cid := logc.OutputToString()

		results := podmanTest.Podman([]string{"logs", "--tail", "2", "-t", cid})
		results.WaitWithDefaultTimeout()
		Expect(results).To(Exit(0))
		Expect(len(results.OutputToStringArray())).To(Equal(2))
	})

	It("since time 2017-08-07", func() {
		logc := podmanTest.Podman([]string{"run", "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(Exit(0))
		cid := logc.OutputToString()

		results := podmanTest.Podman([]string{"logs", "--since", "2017-08-07T10:10:09.056611202-04:00", cid})
		results.WaitWithDefaultTimeout()
		Expect(results).To(Exit(0))
		Expect(len(results.OutputToStringArray())).To(Equal(3))
	})

	It("since duration 10m", func() {
		logc := podmanTest.Podman([]string{"run", "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(Exit(0))
		cid := logc.OutputToString()

		results := podmanTest.Podman([]string{"logs", "--since", "10m", cid})
		results.WaitWithDefaultTimeout()
		Expect(results).To(Exit(0))
		Expect(len(results.OutputToStringArray())).To(Equal(3))
	})

	It("latest and container name should fail", func() {
		SkipIfRemote() // -l not supported
		results := podmanTest.Podman([]string{"logs", "-l", "foobar"})
		results.WaitWithDefaultTimeout()
		Expect(results).To(ExitWithError())
	})

	It("two containers showing short container IDs", func() {
		SkipIfRemote() // remote does not support multiple containers
		log1 := podmanTest.Podman([]string{"run", "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
		log1.WaitWithDefaultTimeout()
		Expect(log1.ExitCode()).To(Equal(0))
		cid1 := log1.OutputToString()

		log2 := podmanTest.Podman([]string{"run", "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
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

	It("podman logs on a created container should result in 0 exit code", func() {
		session := podmanTest.Podman([]string{"create", "-dt", "--name", "log", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).To(Exit(0))

		results := podmanTest.Podman([]string{"logs", "log"})
		results.WaitWithDefaultTimeout()
		Expect(results).To(Exit(0))
	})

	It("using journald for container with container tag", func() {
		SkipIfRemote()
		Skip("need to verify images have correct packages for journald")
		logc := podmanTest.Podman([]string{"run", "--log-driver", "journald", "--log-opt=tag={{.ImageName}}", "-d", ALPINE, "sh", "-c", "echo podman; sleep 0.1; echo podman; sleep 0.1; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(Exit(0))
		cid := logc.OutputToString()

		wait := podmanTest.Podman([]string{"wait", "-l"})
		wait.WaitWithDefaultTimeout()
		Expect(wait).To(Exit(0))

		cmd := exec.Command("journalctl", "--no-pager", "-o", "json", "--output-fields=CONTAINER_TAG", "-u", fmt.Sprintf("libpod-conmon-%s.scope", cid))
		out, err := cmd.CombinedOutput()
		Expect(err).To(BeNil())
		Expect(string(out)).To(ContainSubstring("alpine"))
	})

	It("using journald for container name", func() {
		Skip("need to verify images have correct packages for journald")
		SkipIfRemote()
		containerName := "inside-journal"
		logc := podmanTest.Podman([]string{"run", "--log-driver", "journald", "-d", "--name", containerName, ALPINE, "sh", "-c", "echo podman; sleep 0.1; echo podman; sleep 0.1; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(Exit(0))
		cid := logc.OutputToString()

		wait := podmanTest.Podman([]string{"wait", "-l"})
		wait.WaitWithDefaultTimeout()
		Expect(wait).To(Exit(0))

		cmd := exec.Command("journalctl", "--no-pager", "-o", "json", "--output-fields=CONTAINER_NAME", "-u", fmt.Sprintf("libpod-conmon-%s.scope", cid))
		out, err := cmd.CombinedOutput()
		Expect(err).To(BeNil())
		Expect(string(out)).To(ContainSubstring(containerName))
	})

	It("using journald for container", func() {
		Skip("need to verify images have correct packages for journald")
		logc := podmanTest.Podman([]string{"run", "--log-driver", "journald", "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(Exit(0))
		cid := logc.OutputToString()

		results := podmanTest.Podman([]string{"logs", cid})
		results.WaitWithDefaultTimeout()
		Expect(results).To(Exit(0))
		Expect(len(results.OutputToStringArray())).To(Equal(3))
	})

	It("using journald tail two lines", func() {
		Skip("need to verify images have correct packages for journald")
		logc := podmanTest.Podman([]string{"run", "--log-driver", "journald", "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(Exit(0))
		cid := logc.OutputToString()
		results := podmanTest.Podman([]string{"logs", "--tail", "2", cid})
		results.WaitWithDefaultTimeout()
		Expect(results).To(Exit(0))
		Expect(len(results.OutputToStringArray())).To(Equal(2))
	})

	It("using journald tail 99 lines", func() {
		Skip("need to verify images have correct packages for journald")
		logc := podmanTest.Podman([]string{"run", "--log-driver", "journald", "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(Exit(0))
		cid := logc.OutputToString()

		results := podmanTest.Podman([]string{"logs", "--tail", "99", cid})
		results.WaitWithDefaultTimeout()
		Expect(results).To(Exit(0))
		Expect(len(results.OutputToStringArray())).To(Equal(3))
	})

	It("using journald tail 2 lines with timestamps", func() {
		Skip("need to verify images have correct packages for journald")
		logc := podmanTest.Podman([]string{"run", "--log-driver", "journald", "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(Exit(0))
		cid := logc.OutputToString()

		results := podmanTest.Podman([]string{"logs", "--tail", "2", "-t", cid})
		results.WaitWithDefaultTimeout()
		Expect(results).To(Exit(0))
		Expect(len(results.OutputToStringArray())).To(Equal(2))
	})

	It("using journald since time 2017-08-07", func() {
		Skip("need to verify images have correct packages for journald")
		logc := podmanTest.Podman([]string{"run", "--log-driver", "journald", "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(Exit(0))
		cid := logc.OutputToString()

		results := podmanTest.Podman([]string{"logs", "--since", "2017-08-07T10:10:09.056611202-04:00", cid})
		results.WaitWithDefaultTimeout()
		Expect(results).To(Exit(0))
		Expect(len(results.OutputToStringArray())).To(Equal(3))
	})

	It("using journald with duration 10m", func() {
		Skip("need to verify images have correct packages for journald")
		logc := podmanTest.Podman([]string{"run", "--log-driver", "journald", "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(Exit(0))
		cid := logc.OutputToString()

		results := podmanTest.Podman([]string{"logs", "--since", "10m", cid})
		results.WaitWithDefaultTimeout()
		Expect(results).To(Exit(0))
		Expect(len(results.OutputToStringArray())).To(Equal(3))
	})

	It("streaming output", func() {
		containerName := "logs-f-rm"

		logc := podmanTest.Podman([]string{"run", "--rm", "--name", containerName, "-dt", ALPINE, "sh", "-c", "echo podman; sleep 1; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(Exit(0))

		results := podmanTest.Podman([]string{"logs", "-f", containerName})
		results.WaitWithDefaultTimeout()
		Expect(results).To(Exit(0))

		// Verify that the cleanup process worked correctly and we can recreate a container with the same name
		logc = podmanTest.Podman([]string{"run", "--rm", "--name", containerName, "-dt", ALPINE, "true"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(Exit(0))
	})
})
