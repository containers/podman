package integration

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"

	. "github.com/containers/podman/v4/test/utils"
	"github.com/containers/storage/pkg/stringid"
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
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman ps no containers", func() {
		session := podmanTest.Podman([]string{"ps"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman container ps no containers", func() {
		session := podmanTest.Podman([]string{"container", "ps"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman ps default", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		result := podmanTest.Podman([]string{"ps"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()).ShouldNot(BeEmpty())
	})

	It("podman ps all", func() {
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-a"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()).ShouldNot(BeEmpty())
	})

	It("podman container list all", func() {
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"container", "list", "-a"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()).ShouldNot(BeEmpty())

		result = podmanTest.Podman([]string{"container", "ls", "-a"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()).ShouldNot(BeEmpty())
	})

	It("podman ps size flag", func() {
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-a", "--size"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()).ShouldNot(BeEmpty())
	})

	It("podman ps quiet flag", func() {
		_, ec, fullCid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-a", "-q"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()).ShouldNot(BeEmpty())
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
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman ps last flag", func() {
		Skip("--last flag nonfunctional and disabled")

		// Make sure that non-running containers are being counted as
		// well.
		session := podmanTest.Podman([]string{"create", "alpine", "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		result := podmanTest.Podman([]string{"ps", "--last", "2"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()).Should(HaveLen(2)) // 1 container

		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))

		_, ec, _ = podmanTest.RunLsContainer("test2")
		Expect(ec).To(Equal(0))

		_, ec, _ = podmanTest.RunLsContainer("test3")
		Expect(ec).To(Equal(0))

		result = podmanTest.Podman([]string{"ps", "--last", "2"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()).Should(HaveLen(3)) // 2 containers

		result = podmanTest.Podman([]string{"ps", "--last", "3"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()).Should(HaveLen(4)) // 3 containers

		result = podmanTest.Podman([]string{"ps", "--last", "100"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()).Should(HaveLen(5)) // 4 containers (3 running + 1 created)
	})

	It("podman ps no-trunc", func() {
		_, ec, fullCid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-aq", "--no-trunc"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()).ShouldNot(BeEmpty())
		Expect(fullCid).To(Equal(result.OutputToStringArray()[0]))
	})

	It("podman ps --filter network=container:<name>", func() {
		ctrAlpha := "alpha"
		container := podmanTest.Podman([]string{"run", "-dt", "--name", ctrAlpha, ALPINE, "top"})
		container.WaitWithDefaultTimeout()
		Expect(container).Should(Exit(0))

		ctrBravo := "bravo"
		containerBravo := podmanTest.Podman([]string{"run", "-dt", "--network", "container:alpha", "--name", ctrBravo, ALPINE, "top"})
		containerBravo.WaitWithDefaultTimeout()
		Expect(containerBravo).Should(Exit(0))

		result := podmanTest.Podman([]string{"ps", "-a", "--format", "table {{.Names}}", "--filter", "network=container:alpha"})
		result.WaitWithDefaultTimeout()
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		actual := result.OutputToString()
		Expect(actual).To(ContainSubstring("bravo"))
		Expect(actual).To(ContainSubstring("NAMES"))
	})

	It("podman ps --filter network=container:<id>", func() {
		ctrAlpha := "first"
		container := podmanTest.Podman([]string{"run", "-dt", "--name", ctrAlpha, ALPINE, "top"})
		container.WaitWithDefaultTimeout()
		cid := container.OutputToString()
		Expect(container).Should(Exit(0))

		ctrBravo := "second"
		containerBravo := podmanTest.Podman([]string{"run", "-dt", "--network", "container:" + cid, "--name", ctrBravo, ALPINE, "top"})
		containerBravo.WaitWithDefaultTimeout()
		Expect(containerBravo).Should(Exit(0))

		result := podmanTest.Podman([]string{"ps", "-a", "--format", "table {{.Names}}", "--filter", "network=container:" + cid})
		result.WaitWithDefaultTimeout()
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		actual := result.OutputToString()
		Expect(actual).To(ContainSubstring("second"))
		Expect(actual).ToNot(ContainSubstring("table"))
	})

	It("podman ps namespace flag", func() {
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-a", "--namespace"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()).ShouldNot(BeEmpty())
	})

	It("podman ps namespace flag even for remote", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()

		result := podmanTest.Podman([]string{"ps", "-a", "--ns", "--format",
			"{{with .Namespaces}}{{.Cgroup}}:{{.IPC}}:{{.MNT}}:{{.NET}}:{{.PIDNS}}:{{.User}}:{{.UTS}}{{end}}"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		// it must contains `::` when some ns is null. If it works normally, it should be "$num1:$num2:$num3"
		Expect(result.OutputToString()).ToNot(ContainSubstring(`::`))
	})

	It("podman ps with no containers is valid json format", func() {
		result := podmanTest.Podman([]string{"ps", "--format", "json"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(BeValidJSON())
	})

	It("podman ps namespace flag with json format", func() {
		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-a", "--ns", "--format", "{{ json . }}"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(BeValidJSON())
		// https://github.com/containers/podman/issues/16436
		Expect(result.OutputToString()).To(HavePrefix("{"), "test for single json object and not array see #16436")
	})

	It("podman ps json format Created field is int64", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		result := podmanTest.Podman([]string{"ps", "--format", "json"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		// Make sure Created field is an int64
		created, err := result.jq(".[0].Created")
		Expect(err).ToNot(HaveOccurred())
		_, err = strconv.ParseInt(created, 10, 64)
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman ps print a human-readable `Status` with json format", func() {
		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-a", "--format", "json"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(BeValidJSON())
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
		Expect(result).Should(Exit(0))

		Expect(result.OutputToString()).ToNot(ContainSubstring("table"))

		actual := result.OutputToStringArray()
		Expect(actual[0]).To(ContainSubstring("CONTAINER ID"))
		Expect(actual[0]).ToNot(ContainSubstring("ImageID"))
		Expect(actual[1]).To(ContainSubstring("alpine:latest"))
	})

	It("podman ps ancestor filter flag", func() {
		_, ec, cid := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-q", "--no-trunc", "-a", "--filter", "ancestor=quay.io/libpod/alpine:latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(Equal(cid))

		// Query just by image name, without :latest tag
		result = podmanTest.Podman([]string{"ps", "-q", "--no-trunc", "-a", "--filter", "ancestor=quay.io/libpod/alpine"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(Equal(cid))

		// Query by truncated image name should match (regexp match)
		result = podmanTest.Podman([]string{"ps", "-q", "--no-trunc", "-a", "--filter", "ancestor=quay.io/libpod/alpi"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(Equal(cid))

		// Query using regex by truncated image name should match (regexp match)
		result = podmanTest.Podman([]string{"ps", "-q", "--no-trunc", "-a", "--filter", "ancestor=^(quay.io|docker.io)/libpod/alpine:[a-zA-Z]+"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(Equal(cid))

		// Query for an non-existing image using regex should not match anything
		result = podmanTest.Podman([]string{"ps", "-q", "--no-trunc", "-a", "--filter", "ancestor=^quai.io/libpod/alpi"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(Equal(""))
	})

	It("podman ps id filter flag", func() {
		_, ec, fullCid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-a", "--filter", fmt.Sprintf("id=%s", fullCid)})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
	})

	It("podman ps id filter flag", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		fullCid := session.OutputToString()

		result := podmanTest.Podman([]string{"ps", "-aq", "--no-trunc", "--filter", "status=running"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()[0]).To(Equal(fullCid))
	})

	It("podman ps multiple filters", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--name", "test1", "--label", "key1=value1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		fullCid := session.OutputToString()

		session2 := podmanTest.Podman([]string{"run", "-d", "--name", "test2", "--label", "key1=value1", ALPINE, "top"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))

		result := podmanTest.Podman([]string{"ps", "-aq", "--no-trunc", "--filter", "name=test1", "--filter", "label=key1=value1"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		output := result.OutputToStringArray()
		Expect(output).To(HaveLen(1))
		Expect(output[0]).To(Equal(fullCid))
	})

	It("podman ps filter by exited does not need all", func() {
		ctr := podmanTest.Podman([]string{"run", "-t", "-i", ALPINE, "ls", "/"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(Exit(0))

		psAll := podmanTest.Podman([]string{"ps", "-aq", "--no-trunc"})
		psAll.WaitWithDefaultTimeout()
		Expect(psAll).Should(Exit(0))

		psFilter := podmanTest.Podman([]string{"ps", "--no-trunc", "--quiet", "--filter", "status=exited"})
		psFilter.WaitWithDefaultTimeout()
		Expect(psFilter).Should(Exit(0))

		Expect(psAll.OutputToString()).To(Equal(psFilter.OutputToString()))
	})

	It("podman filter without status does not find non-running", func() {
		ctrName := "aContainerName"
		ctr := podmanTest.Podman([]string{"create", "--name", ctrName, "-t", "-i", ALPINE, "ls", "/"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(Exit(0))

		psFilter := podmanTest.Podman([]string{"ps", "--no-trunc", "--quiet", "--format", "{{.Names}}", "--filter", fmt.Sprintf("name=%s", ctrName)})
		psFilter.WaitWithDefaultTimeout()
		Expect(psFilter).Should(Exit(0))

		actual := psFilter.OutputToString()
		Expect(actual).ToNot(ContainSubstring(ctrName))
		Expect(actual).ToNot(ContainSubstring("NAMES"))
	})

	It("podman ps mutually exclusive flags", func() {
		session := podmanTest.Podman([]string{"ps", "-aqs"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"ps", "-a", "--ns", "-s"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman --format by size", func() {
		session := podmanTest.Podman([]string{"create", BB, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"create", "-t", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"ps", "-a", "--format", "{{.Size}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.ErrorToString()).To(ContainSubstring("Size format requires --size option"))
	})

	It("podman --sort by size", func() {
		session := podmanTest.Podman([]string{"create", BB, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"create", "-t", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"ps", "-a", "-s", "--sort=size", "--format", "{{.Size}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

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
			}
			size1, _ := units.FromHumanSize(matches1[1])
			size2, _ := units.FromHumanSize(matches2[1])
			return size1 < size2
		})).To(BeTrue())

	})

	It("podman --sort by command", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "-d", ALPINE, "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"ps", "-a", "--sort=command", "--format", "{{.Command}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		Expect(session.OutputToString()).ToNot(ContainSubstring("COMMAND"))

		sortedArr := session.OutputToStringArray()
		Expect(sort.SliceIsSorted(sortedArr, func(i, j int) bool { return sortedArr[i] < sortedArr[j] })).To(BeTrue())
	})

	It("podman --pod", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"ps", "--no-trunc"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).ToNot(ContainSubstring(podid))

		session = podmanTest.Podman([]string{"ps", "--pod", "--no-trunc"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(podid))
	})

	It("podman --pod with a non-empty pod name", func() {
		podName := "testPodName"
		_, ec, podid := podmanTest.CreatePod(map[string][]string{"--name": {podName}})
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podName)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// "--no-trunc" must be given. If not it will trunc the pod ID
		// in the output and you will have to trunc it in the test too.
		session = podmanTest.Podman([]string{"ps", "--pod", "--no-trunc"})

		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		output := session.OutputToString()
		Expect(output).To(ContainSubstring(podid))
		Expect(output).To(ContainSubstring(podName))
	})

	It("podman ps test with single port range", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "-p", "2000-2006:2000-2006", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"ps", "--format", "{{.Ports}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(Exit(0))

		actual := session.OutputToString()
		Expect(actual).To(ContainSubstring("0.0.0.0:2000-2006"))
		Expect(actual).ToNot(ContainSubstring("PORT"))
	})

	It("podman ps test with invalid port range", func() {
		session := podmanTest.Podman([]string{
			"run", "-p", "1000-2000:2000-3000", "-p", "1999-2999:3001-4001", ALPINE,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
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
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"ps", "--format", "{{.Ports}}"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(ContainSubstring(
			"0.0.0.0:3000-3001->3000-3001/tcp, 0.0.0.0:3100-3102->4000-4002/tcp, 0.0.0.0:8000->8080/tcp, 0.0.0.0:30080->30080/tcp, 0.0.0.0:30443->30443/tcp",
		))
	})

	It("podman ps sync flag", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		fullCid := session.OutputToString()

		result := podmanTest.Podman([]string{"ps", "-q", "--no-trunc", "--sync"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()[0]).To(Equal(fullCid))
	})

	It("podman ps filter name regexp", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--name", "test1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		fullCid := session.OutputToString()

		session2 := podmanTest.Podman([]string{"run", "-d", "--name", "test11", ALPINE, "top"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))

		result := podmanTest.Podman([]string{"ps", "-aq", "--no-trunc", "--filter", "name=test1"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		output := result.OutputToStringArray()
		Expect(output).To(HaveLen(2))

		result = podmanTest.Podman([]string{"ps", "-aq", "--no-trunc", "--filter", "name=test1$"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		output = result.OutputToStringArray()
		Expect(output).To(HaveLen(1))
		Expect(output[0]).To(Equal(fullCid))
	})

	It("podman ps quiet template", func() {
		ctrName := "testCtr"
		session := podmanTest.Podman([]string{"run", "-d", "--name", ctrName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		result := podmanTest.Podman([]string{"ps", "-q", "-a", "--format", "{{ .Names }}"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		output := result.OutputToStringArray()
		Expect(output).To(HaveLen(1))
		Expect(output[0]).To(Equal(ctrName))
	})

	It("podman ps test with port shared with pod", func() {
		podName := "testPod"
		pod := podmanTest.Podman([]string{"pod", "create", "-p", "8085:80", "--name", podName})
		pod.WaitWithDefaultTimeout()
		Expect(pod).Should(Exit(0))

		ctrName := "testCtr"
		session := podmanTest.Podman([]string{"run", "--name", ctrName, "-dt", "--pod", podName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		ps := podmanTest.Podman([]string{"ps", "--filter", fmt.Sprintf("name=%s", ctrName), "--format", "{{.Ports}}"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(Exit(0))
		Expect(ps.OutputToString()).To(ContainSubstring("0.0.0.0:8085->80/tcp"))
	})

	It("podman ps truncate long create command", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "echo", "very", "long", "create", "command"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"ps", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(ContainSubstring("echo very long cr..."))
	})
	It("podman ps --format {{RunningFor}}", func() {
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"ps", "-a", "--format", "{{.RunningFor}}"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		actual := result.OutputToString()
		Expect(actual).To(ContainSubstring("ago"))
		Expect(actual).ToNot(ContainSubstring("RUNNING FOR"))
	})

	It("podman ps filter test", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--name", "test1", "--label", "foo=1",
			"--label", "bar=2", "--volume", "volume1:/test", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid1 := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--name", "test2", "--label", "foo=1",
			ALPINE, "ls", "/fail"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))

		session = podmanTest.Podman([]string{"create", "--name", "test3", ALPINE, cid1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "--name", "test4", "--volume", "volume1:/test1",
			"--volume", "/:/test2", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"ps", "--all", "--filter", "name=test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(5))
		Expect(session.OutputToString()).To(ContainSubstring("test1"))
		Expect(session.OutputToString()).To(ContainSubstring("test2"))
		Expect(session.OutputToString()).To(ContainSubstring("test3"))
		Expect(session.OutputToString()).To(ContainSubstring("test4"))

		session = podmanTest.Podman([]string{"ps", "--all", "--filter", "name=test1", "--filter", "name=test2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(3))
		Expect(session.OutputToString()).To(ContainSubstring("test1"))
		Expect(session.OutputToString()).To(ContainSubstring("test2"))

		// check container id matches with regex
		session = podmanTest.Podman([]string{"ps", "--all", "--filter", "id=" + cid1[:40], "--filter", "id=" + cid1 + "$"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToString()).To(ContainSubstring("test1"))

		session = podmanTest.Podman([]string{"ps", "--filter", "status=created"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToString()).To(ContainSubstring("test3"))

		session = podmanTest.Podman([]string{"ps", "--filter", "status=created", "--filter", "status=exited"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(4))
		Expect(session.OutputToString()).To(ContainSubstring("test2"))
		Expect(session.OutputToString()).To(ContainSubstring("test3"))
		Expect(session.OutputToString()).To(ContainSubstring("test4"))

		session = podmanTest.Podman([]string{"ps", "--all", "--filter", "label=foo=1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(3))
		Expect(session.OutputToString()).To(ContainSubstring("test1"))
		Expect(session.OutputToString()).To(ContainSubstring("test2"))

		session = podmanTest.Podman([]string{"ps", "--filter", "label=foo=1", "--filter", "status=exited"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToString()).To(ContainSubstring("test2"))

		session = podmanTest.Podman([]string{"ps", "--all", "--filter", "label=foo=1", "--filter", "label=non=1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(1))

		session = podmanTest.Podman([]string{"ps", "--all", "--filter", "label=foo=1", "--filter", "label=bar=2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToString()).To(ContainSubstring("test1"))

		session = podmanTest.Podman([]string{"ps", "--all", "--filter", "exited=1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToString()).To(ContainSubstring("test2"))

		session = podmanTest.Podman([]string{"ps", "--all", "--filter", "exited=1", "--filter", "exited=0"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(3))
		Expect(session.OutputToString()).To(ContainSubstring("test2"))
		Expect(session.OutputToString()).To(ContainSubstring("test4"))

		session = podmanTest.Podman([]string{"ps", "--all", "--filter", "volume=volume1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(3))
		Expect(session.OutputToString()).To(ContainSubstring("test1"))
		Expect(session.OutputToString()).To(ContainSubstring("test4"))

		session = podmanTest.Podman([]string{"ps", "--all", "--filter", "volume=/:/test2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToString()).To(ContainSubstring("test4"))

		session = podmanTest.Podman([]string{"ps", "--all", "--filter", "before=test2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToString()).To(ContainSubstring("test1"))

		session = podmanTest.Podman([]string{"ps", "--all", "--filter", "since=test2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(3))
		Expect(session.OutputToString()).To(ContainSubstring("test3"))
		Expect(session.OutputToString()).To(ContainSubstring("test4"))
	})
	It("podman ps filter pod", func() {
		pod1 := podmanTest.Podman([]string{"pod", "create", "--name", "pod1"})
		pod1.WaitWithDefaultTimeout()
		Expect(pod1).Should(Exit(0))
		con1 := podmanTest.Podman([]string{"run", "-dt", "--pod", "pod1", ALPINE, "top"})
		con1.WaitWithDefaultTimeout()
		Expect(con1).Should(Exit(0))

		pod2 := podmanTest.Podman([]string{"pod", "create", "--name", "pod2"})
		pod2.WaitWithDefaultTimeout()
		Expect(pod2).Should(Exit(0))
		con2 := podmanTest.Podman([]string{"run", "-dt", "--pod", "pod2", ALPINE, "top"})
		con2.WaitWithDefaultTimeout()
		Expect(con2).Should(Exit(0))

		// bogus pod name or id should not result in error
		session := podmanTest.Podman([]string{"ps", "--filter", "pod=1234"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// filter by pod name
		session = podmanTest.Podman([]string{"ps", "-q", "--no-trunc", "--filter", "pod=pod1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToStringArray()).To(ContainElement(con1.OutputToString()))

		// filter by full pod id
		session = podmanTest.Podman([]string{"ps", "-q", "--no-trunc", "--filter", "pod=" + pod1.OutputToString()})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToStringArray()).To(ContainElement(con1.OutputToString()))

		// filter by partial pod id
		session = podmanTest.Podman([]string{"ps", "-q", "--no-trunc", "--filter", "pod=" + pod1.OutputToString()[0:12]})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToStringArray()).To(ContainElement(con1.OutputToString()))

		// filter by multiple pods is inclusive
		session = podmanTest.Podman([]string{"ps", "-q", "--no-trunc", "--filter", "pod=pod1", "--filter", "pod=pod2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(4))
		Expect(session.OutputToStringArray()).To(ContainElement(con1.OutputToString()))
		Expect(session.OutputToStringArray()).To(ContainElement(con2.OutputToString()))

	})

	It("podman ps filter network", func() {
		net := stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", net})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		defer podmanTest.removeNetwork(net)

		session = podmanTest.Podman([]string{"create", "--network", net, ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		ctrWithNet := session.OutputToString()

		session = podmanTest.Podman([]string{"create", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		ctrWithoutNet := session.OutputToString()

		session = podmanTest.Podman([]string{"ps", "--all", "--no-trunc", "--filter", "network=" + net})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		actual := session.OutputToString()
		Expect(actual).To(ContainSubstring(ctrWithNet))
		Expect(actual).ToNot(ContainSubstring(ctrWithoutNet))
	})

	It("podman ps --format networks", func() {
		session := podmanTest.Podman([]string{"create", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"ps", "--all", "--format", "{{ .Networks }}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		actual := session.OutputToString()
		Expect(actual).ToNot(ContainSubstring("NETWORKS"))
		if isRootless() {
			// rootless container don't have a network by default
			Expect(actual).To(BeEmpty())
		} else {
			// default network name is podman
			Expect(actual).To(Equal("podman"))
		}

		net1 := stringid.GenerateRandomID()
		session = podmanTest.Podman([]string{"network", "create", net1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		defer podmanTest.removeNetwork(net1)
		net2 := stringid.GenerateRandomID()
		session = podmanTest.Podman([]string{"network", "create", net2})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		defer podmanTest.removeNetwork(net2)

		session = podmanTest.Podman([]string{"create", "--network", net1 + "," + net2, ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{"ps", "--all", "--format", "{{ .Networks }}", "--filter", "id=" + cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		// the output is not deterministic so check both possible orders
		Expect(session.OutputToString()).To(Or(Equal(net1+","+net2), Equal(net2+","+net1)))
	})

})
