package integration

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	. "github.com/containers/podman/v2/test/utils"
	"github.com/docker/go-units"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman ps", func() {
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

	It("podman ps no containers", func() {
		session := podmanTest.Podman([]string{"ps"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman ps default", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"ps"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).Should(BeNumerically(">", 0))
	})

	It("podman ps all", func() {
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-a"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).Should(BeNumerically(">", 0))
	})

	It("podman container list all", func() {
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"container", "list", "-a"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).Should(BeNumerically(">", 0))

		result = podmanTest.Podman([]string{"container", "ls", "-a"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).Should(BeNumerically(">", 0))
	})

	It("podman ps size flag", func() {
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-a", "--size"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).Should(BeNumerically(">", 0))
	})

	It("podman ps quiet flag", func() {
		_, ec, fullCid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-a", "-q"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).Should(BeNumerically(">", 0))
		Expect(fullCid).To(ContainSubstring(result.OutputToStringArray()[0]))
	})

	It("podman ps latest flag", func() {
		SkipIfRemote("--latest is not supported on podman-remote")
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		_, ec, _ = podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-q", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).Should(Equal(1))
	})

	It("podman ps last flag", func() {
		Skip("--last flag nonfunctional and disabled")

		// Make sure that non-running containers are being counted as
		// well.
		session := podmanTest.Podman([]string{"create", "alpine", "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "--last", "2"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).Should(Equal(2)) // 1 container

		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))

		_, ec, _ = podmanTest.RunLsContainer("test2")
		Expect(ec).To(Equal(0))

		_, ec, _ = podmanTest.RunLsContainer("test3")
		Expect(ec).To(Equal(0))

		result = podmanTest.Podman([]string{"ps", "--last", "2"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).Should(Equal(3)) // 2 containers

		result = podmanTest.Podman([]string{"ps", "--last", "3"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).Should(Equal(4)) // 3 containers

		result = podmanTest.Podman([]string{"ps", "--last", "100"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).Should(Equal(5)) // 4 containers (3 running + 1 created)
	})

	It("podman ps no-trunc", func() {
		_, ec, fullCid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-aq", "--no-trunc"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).Should(BeNumerically(">", 0))
		Expect(fullCid).To(Equal(result.OutputToStringArray()[0]))
	})

	It("podman ps namespace flag", func() {
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-a", "--namespace"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).Should(BeNumerically(">", 0))
	})

	It("podman ps namespace flag even for remote", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()

		result := podmanTest.Podman([]string{"ps", "-a", "--namespace", "--format",
			"{{with .Namespaces}}{{.Cgroup}}:{{.IPC}}:{{.MNT}}:{{.NET}}:{{.PIDNS}}:{{.User}}:{{.UTS}}{{end}}"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		// it must contains `::` when some ns is null. If it works normally, it should be "$num1:$num2:$num3"
		Expect(result.OutputToString()).To(Not(ContainSubstring(`::`)))
	})

	It("podman ps with no containers is valid json format", func() {
		result := podmanTest.Podman([]string{"ps", "--format", "json"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.IsJSONOutputValid()).To(BeTrue())
	})

	It("podman ps namespace flag with json format", func() {
		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-a", "--ns", "--format", "json"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.IsJSONOutputValid()).To(BeTrue())
	})

	It("podman ps print a human-readable `Status` with json format", func() {
		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-a", "--format", "json"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.IsJSONOutputValid()).To(BeTrue())
		// must contain "Status"
		match, StatusLine := result.GrepString(`Status`)
		Expect(match).To(BeTrue())
		// container is running or exit, so it must contain `ago`
		Expect(StatusLine[0]).To(ContainSubstring("ago"))
	})

	It("podman ps namespace flag with go template format", func() {
		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-a", "--format", "table {{.ID}} {{.Image}} {{.ImageID}} {{.Labels}}"})
		result.WaitWithDefaultTimeout()

		Expect(result.OutputToStringArray()[0]).ToNot(ContainSubstring("table"))
		Expect(result.OutputToStringArray()[0]).ToNot(ContainSubstring("ImageID"))
		Expect(result.OutputToStringArray()[0]).To(ContainSubstring("alpine:latest"))
		Expect(result).Should(Exit(0))
	})

	It("podman ps ancestor filter flag", func() {
		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-a", "--filter", "ancestor=docker.io/library/alpine:latest"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman ps id filter flag", func() {
		_, ec, fullCid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-a", "--filter", fmt.Sprintf("id=%s", fullCid)})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman ps id filter flag", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		fullCid := session.OutputToString()

		result := podmanTest.Podman([]string{"ps", "-aq", "--no-trunc", "--filter", "status=running"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToStringArray()[0]).To(Equal(fullCid))
	})

	It("podman ps multiple filters", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--name", "test1", "--label", "key1=value1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		fullCid := session.OutputToString()

		session2 := podmanTest.Podman([]string{"run", "-d", "--name", "test2", "--label", "key1=value1", ALPINE, "top"})
		session2.WaitWithDefaultTimeout()
		Expect(session2.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-aq", "--no-trunc", "--filter", "name=test1", "--filter", "label=key1=value1"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		output := result.OutputToStringArray()
		Expect(len(output)).To(Equal(1))
		Expect(output[0]).To(Equal(fullCid))
	})

	It("podman ps filter by exited does not need all", func() {
		ctr := podmanTest.Podman([]string{"run", "-t", "-i", ALPINE, "ls", "/"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr.ExitCode()).To(Equal(0))

		psAll := podmanTest.Podman([]string{"ps", "-aq", "--no-trunc"})
		psAll.WaitWithDefaultTimeout()
		Expect(psAll.ExitCode()).To(Equal(0))

		psFilter := podmanTest.Podman([]string{"ps", "--no-trunc", "--quiet", "--filter", "status=exited"})
		psFilter.WaitWithDefaultTimeout()
		Expect(psFilter.ExitCode()).To(Equal(0))

		Expect(psAll.OutputToString()).To(Equal(psFilter.OutputToString()))
	})

	It("podman filter without status does not find non-running", func() {
		ctrName := "aContainerName"
		ctr := podmanTest.Podman([]string{"create", "--name", ctrName, "-t", "-i", ALPINE, "ls", "/"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr.ExitCode()).To(Equal(0))

		psFilter := podmanTest.Podman([]string{"ps", "--no-trunc", "--quiet", "--format", "{{.Names}}", "--filter", fmt.Sprintf("name=%s", ctrName)})
		psFilter.WaitWithDefaultTimeout()
		Expect(psFilter.ExitCode()).To(Equal(0))

		Expect(strings.Contains(psFilter.OutputToString(), ctrName)).To(BeFalse())
	})

	It("podman ps mutually exclusive flags", func() {
		session := podmanTest.Podman([]string{"ps", "-aqs"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"ps", "-a", "--ns", "-s"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman --sort by size", func() {
		session := podmanTest.Podman([]string{"create", "busybox", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"create", "-dt", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"ps", "-a", "-s", "--sort=size", "--format", "{{.Size}}"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		sortedArr := session.OutputToStringArray()

		// TODO: This may be broken - the test was running without the
		// ability to perform any sorting for months and succeeded
		// without error.
		Expect(sort.SliceIsSorted(sortedArr, func(i, j int) bool {
			r := regexp.MustCompile(`^\S+\s+\(virtual (\S+)\)`)
			matches1 := r.FindStringSubmatch(sortedArr[i])
			matches2 := r.FindStringSubmatch(sortedArr[j])

			// sanity check in case an oddly formatted size appears
			if len(matches1) < 2 || len(matches2) < 2 {
				return sortedArr[i] < sortedArr[j]
			} else {
				size1, _ := units.FromHumanSize(matches1[1])
				size2, _ := units.FromHumanSize(matches2[1])
				return size1 < size2
			}
		})).To(BeTrue())

	})

	It("podman --sort by command", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", "-d", ALPINE, "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"ps", "-a", "--sort=command", "--format", "{{.Command}}"})

		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		sortedArr := session.OutputToStringArray()

		Expect(sort.SliceIsSorted(sortedArr, func(i, j int) bool { return sortedArr[i] < sortedArr[j] })).To(BeTrue())

	})

	It("podman --pod", func() {
		_, ec, podid := podmanTest.CreatePod("")
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"ps", "--pod", "--no-trunc"})

		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		Expect(session.OutputToString()).To(ContainSubstring(podid))
	})

	It("podman --pod with a non-empty pod name", func() {
		podName := "testPodName"
		_, ec, podid := podmanTest.CreatePod(podName)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podName)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// "--no-trunc" must be given. If not it will trunc the pod ID
		// in the output and you will have to trunc it in the test too.
		session = podmanTest.Podman([]string{"ps", "--pod", "--no-trunc"})

		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		output := session.OutputToString()
		Expect(output).To(ContainSubstring(podid))
		Expect(output).To(ContainSubstring(podName))
	})

	It("podman ps test with single port range", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "-p", "2000-2006:2000-2006", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"ps", "--format", "{{.Ports}}"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(ContainSubstring("0.0.0.0:2000-2006"))
	})

	It("podman ps test with invalid port range", func() {
		session := podmanTest.Podman([]string{
			"run", "-p", "1000-2000:2000-3000", "-p", "1999-2999:3001-4001", ALPINE,
		})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
		Expect(session.ErrorToString()).To(ContainSubstring("conflicting port mappings for host port 1999"))
	})

	It("podman ps test with multiple port range", func() {
		session := podmanTest.Podman([]string{
			"run", "-dt",
			"-p", "3000-3001:3000-3001",
			"-p", "3100-3102:4000-4002",
			"-p", "30080:30080",
			"-p", "30443:30443",
			"-p", "8000:8080",
			ALPINE, "top"},
		)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"ps", "--format", "{{.Ports}}"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(ContainSubstring(
			"0.0.0.0:3000-3001->3000-3001/tcp, 0.0.0.0:3100-3102->4000-4002/tcp, 0.0.0.0:8000->8080/tcp, 0.0.0.0:30080->30080/tcp, 0.0.0.0:30443->30443/tcp",
		))
	})

	It("podman ps sync flag", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		fullCid := session.OutputToString()

		result := podmanTest.Podman([]string{"ps", "-q", "--no-trunc", "--sync"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToStringArray()[0]).To(Equal(fullCid))
	})

	It("podman ps filter name regexp", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--name", "test1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		fullCid := session.OutputToString()

		session2 := podmanTest.Podman([]string{"run", "-d", "--name", "test11", ALPINE, "top"})
		session2.WaitWithDefaultTimeout()
		Expect(session2.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-aq", "--no-trunc", "--filter", "name=test1"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		output := result.OutputToStringArray()
		Expect(len(output)).To(Equal(2))

		result = podmanTest.Podman([]string{"ps", "-aq", "--no-trunc", "--filter", "name=test1$"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		output = result.OutputToStringArray()
		Expect(len(output)).To(Equal(1))
		Expect(output[0]).To(Equal(fullCid))
	})

	It("podman ps quiet template", func() {
		ctrName := "testCtr"
		session := podmanTest.Podman([]string{"run", "-d", "--name", ctrName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-q", "-a", "--format", "{{ .Names }}"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		output := result.OutputToStringArray()
		Expect(len(output)).To(Equal(1))
		Expect(output[0]).To(Equal(ctrName))
	})

	It("podman ps test with port shared with pod", func() {
		podName := "testPod"
		pod := podmanTest.Podman([]string{"pod", "create", "-p", "8080:80", "--name", podName})
		pod.WaitWithDefaultTimeout()
		Expect(pod.ExitCode()).To(Equal(0))

		ctrName := "testCtr"
		session := podmanTest.Podman([]string{"run", "--name", ctrName, "-dt", "--pod", podName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		ps := podmanTest.Podman([]string{"ps", "--filter", fmt.Sprintf("name=%s", ctrName), "--format", "{{.Ports}}"})
		ps.WaitWithDefaultTimeout()
		Expect(ps.ExitCode()).To(Equal(0))
		Expect(ps.OutputToString()).To(ContainSubstring("0.0.0.0:8080->80/tcp"))
	})

	It("podman ps truncate long create command", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "echo", "very", "long", "create", "command"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"ps", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(ContainSubstring("echo very long cr..."))
	})
	It("podman ps --format {{RunningFor}}", func() {
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-a", "--format", "{{.RunningFor}}"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(ContainSubstring("ago"))
	})
})
