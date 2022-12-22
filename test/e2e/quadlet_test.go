package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/containers/podman/v4/pkg/systemd/parser"
	"github.com/mattn/go-shellwords"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

type quadletTestcase struct {
	data        []byte
	serviceName string
	checks      [][]string
}

func loadQuadletTestcase(path string) *quadletTestcase {
	data, err := os.ReadFile(path)
	Expect(err).ToNot(HaveOccurred())

	base := filepath.Base(path)
	ext := filepath.Ext(base)
	service := base[:len(base)-len(ext)]
	switch ext {
	case ".volume":
		service += "-volume"
	case ".network":
		service += "-network"
	}
	service += ".service"

	checks := make([][]string, 0)

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "##") {
			words, err := shellwords.Parse(line[2:])
			Expect(err).ToNot(HaveOccurred())
			checks = append(checks, words)
		}
	}

	return &quadletTestcase{
		data,
		service,
		checks,
	}
}

func matchSublistAt(full []string, pos int, sublist []string) bool {
	if len(sublist) > len(full)-pos {
		return false
	}

	for i := range sublist {
		if sublist[i] != full[pos+i] {
			return false
		}
	}
	return true
}

func matchSublistRegexAt(full []string, pos int, sublist []string) bool {
	if len(sublist) > len(full)-pos {
		return false
	}

	for i := range sublist {
		matched, err := regexp.MatchString(sublist[i], full[pos+i])
		if err != nil || !matched {
			return false
		}
	}
	return true
}

func findSublist(full []string, sublist []string) int {
	if len(sublist) > len(full) {
		return -1
	}
	if len(sublist) == 0 {
		return -1
	}
	for i := 0; i < len(full)-len(sublist)+1; i++ {
		if matchSublistAt(full, i, sublist) {
			return i
		}
	}
	return -1
}

func findSublistRegex(full []string, sublist []string) int {
	if len(sublist) > len(full) {
		return -1
	}
	if len(sublist) == 0 {
		return -1
	}
	for i := 0; i < len(full)-len(sublist)+1; i++ {
		if matchSublistRegexAt(full, i, sublist) {
			return i
		}
	}
	return -1
}

func (t *quadletTestcase) assertStdErrContains(args []string, session *PodmanSessionIntegration) bool {
	return strings.Contains(session.ErrorToString(), args[0])
}

func (t *quadletTestcase) assertKeyIs(args []string, unit *parser.UnitFile) bool {
	Expect(len(args)).To(BeNumerically(">=", 3))
	group := args[0]
	key := args[1]
	values := args[2:]

	realValues := unit.LookupAll(group, key)
	if len(realValues) != len(values) {
		return false
	}

	for i := range realValues {
		if realValues[i] != values[i] {
			return false
		}
	}
	return true
}

func (t *quadletTestcase) assertKeyIsRegex(args []string, unit *parser.UnitFile) bool {
	Expect(len(args)).To(BeNumerically(">=", 3))
	group := args[0]
	key := args[1]
	values := args[2:]

	realValues := unit.LookupAll(group, key)
	if len(realValues) != len(values) {
		return false
	}

	for i := range realValues {
		matched, _ := regexp.MatchString(values[i], realValues[i])
		if !matched {
			return false
		}
	}
	return true
}

func (t *quadletTestcase) assertKeyContains(args []string, unit *parser.UnitFile) bool {
	Expect(args).To(HaveLen(3))
	group := args[0]
	key := args[1]
	value := args[2]

	realValue, ok := unit.LookupLast(group, key)
	return ok && strings.Contains(realValue, value)
}

func (t *quadletTestcase) assertPodmanArgs(args []string, unit *parser.UnitFile, key string) bool {
	podmanArgs, _ := unit.LookupLastArgs("Service", key)
	return findSublist(podmanArgs, args) != -1
}

func (t *quadletTestcase) assertPodmanArgsRegex(args []string, unit *parser.UnitFile, key string) bool {
	podmanArgs, _ := unit.LookupLastArgs("Service", key)
	return findSublistRegex(podmanArgs, args) != -1
}

func (t *quadletTestcase) assertPodmanFinalArgs(args []string, unit *parser.UnitFile, key string) bool {
	podmanArgs, _ := unit.LookupLastArgs("Service", key)
	if len(podmanArgs) < len(args) {
		return false
	}
	return matchSublistAt(podmanArgs, len(podmanArgs)-len(args), args)
}

func (t *quadletTestcase) assertPodmanFinalArgsRegex(args []string, unit *parser.UnitFile, key string) bool {
	podmanArgs, _ := unit.LookupLastArgs("Service", key)
	if len(podmanArgs) < len(args) {
		return false
	}
	return matchSublistRegexAt(podmanArgs, len(podmanArgs)-len(args), args)
}

func (t *quadletTestcase) assertStartPodmanArgs(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgs(args, unit, "ExecStart")
}

func (t *quadletTestcase) assertStartPodmanArgsRegex(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgsRegex(args, unit, "ExecStart")
}

func (t *quadletTestcase) assertStartPodmanFinalArgs(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanFinalArgs(args, unit, "ExecStart")
}

func (t *quadletTestcase) assertStartPodmanFinalArgsRegex(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanFinalArgsRegex(args, unit, "ExecStart")
}

func (t *quadletTestcase) assertStopPodmanArgs(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgs(args, unit, "ExecStop")
}

func (t *quadletTestcase) assertStopPodmanFinalArgs(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanFinalArgs(args, unit, "ExecStop")
}

func (t *quadletTestcase) assertStopPodmanFinalArgsRegex(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanFinalArgsRegex(args, unit, "ExecStop")
}

func (t *quadletTestcase) assertSymlink(args []string, unit *parser.UnitFile) bool {
	Expect(args).To(HaveLen(2))
	symlink := args[0]
	expectedTarget := args[1]

	dir := filepath.Dir(unit.Path)

	target, err := os.Readlink(filepath.Join(dir, symlink))
	Expect(err).ToNot(HaveOccurred())

	return expectedTarget == target
}

func (t *quadletTestcase) doAssert(check []string, unit *parser.UnitFile, session *PodmanSessionIntegration) error {
	Expect(check).ToNot(BeEmpty())
	op := check[0]
	args := make([]string, 0)
	for _, a := range check[1:] {
		// Apply \n and \t as they are used in the testcases
		a = strings.ReplaceAll(a, "\\n", "\n")
		a = strings.ReplaceAll(a, "\\t", "\t")
		args = append(args, a)
	}
	invert := false
	if op[0] == '!' {
		invert = true
		op = op[1:]
	}

	var ok bool
	switch op {
	case "assert-failed":
		ok = true /* Handled separately */
	case "assert-stderr-contains":
		ok = t.assertStdErrContains(args, session)
	case "assert-key-is":
		ok = t.assertKeyIs(args, unit)
	case "assert-key-is-regex":
		ok = t.assertKeyIsRegex(args, unit)
	case "assert-key-contains":
		ok = t.assertKeyContains(args, unit)
	case "assert-podman-args":
		ok = t.assertStartPodmanArgs(args, unit)
	case "assert-podman-args-regex":
		ok = t.assertStartPodmanArgsRegex(args, unit)
	case "assert-podman-final-args":
		ok = t.assertStartPodmanFinalArgs(args, unit)
	case "assert-podman-final-args-regex":
		ok = t.assertStartPodmanFinalArgsRegex(args, unit)
	case "assert-symlink":
		ok = t.assertSymlink(args, unit)
	case "assert-podman-stop-args":
		ok = t.assertStopPodmanArgs(args, unit)
	case "assert-podman-stop-final-args":
		ok = t.assertStopPodmanFinalArgs(args, unit)
	case "assert-podman-stop-final-args-regex":
		ok = t.assertStopPodmanFinalArgsRegex(args, unit)
	default:
		return fmt.Errorf("Unsupported assertion %s", op)
	}
	if invert {
		ok = !ok
	}

	if !ok {
		s := "(nil)"
		if unit != nil {
			s, _ = unit.ToString()
		}
		return fmt.Errorf("Failed assertion for %s: %s\n\n%s", t.serviceName, strings.Join(check, " "), s)
	}
	return nil
}

func (t *quadletTestcase) check(generateDir string, session *PodmanSessionIntegration) {
	expectFail := false
	for _, c := range t.checks {
		if c[0] == "assert-failed" {
			expectFail = true
		}
	}

	file := filepath.Join(generateDir, t.serviceName)
	_, err := os.Stat(file)
	if expectFail {
		Expect(err).To(MatchError(os.ErrNotExist))
	} else {
		Expect(err).ToNot(HaveOccurred())
	}

	var unit *parser.UnitFile
	if !expectFail {
		unit, err = parser.ParseUnitFile(file)
		Expect(err).ToNot(HaveOccurred())
	}

	for _, check := range t.checks {
		err := t.doAssert(check, unit, session)
		Expect(err).ToNot(HaveOccurred())
	}
}

var _ = Describe("quadlet system generator", func() {
	var (
		tempdir      string
		err          error
		generatedDir string
		quadletDir   string
		podmanTest   *PodmanTestIntegration
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()

		generatedDir = filepath.Join(podmanTest.TempDir, "generated")
		err = os.Mkdir(generatedDir, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		quadletDir = filepath.Join(podmanTest.TempDir, "quadlet")
		err = os.Mkdir(quadletDir, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	Describe("Running quadlet dryrun tests", func() {
		It("Should exit with an error because of no files are found to parse", func() {
			fileName := "basic.kube"
			testcase := loadQuadletTestcase(filepath.Join("quadlet", fileName))

			// Write the tested file to the quadlet dir
			err = os.WriteFile(filepath.Join(quadletDir, fileName), testcase.data, 0644)
			Expect(err).ToNot(HaveOccurred())

			session := podmanTest.Quadlet([]string{"-dryrun"}, "/something")
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(1))

			current := session.ErrorToStringArray()
			expected := "No files to parse from [/something]"

			Expect(strings.Contains(current[0], expected)).To(BeTrue())
		})

		It("Should parse a kube file and print it to stdout", func() {
			fileName := "basic.kube"
			testcase := loadQuadletTestcase(filepath.Join("quadlet", fileName))

			// Write the tested file to the quadlet dir
			err = os.WriteFile(filepath.Join(quadletDir, fileName), testcase.data, 0644)
			Expect(err).ToNot(HaveOccurred())

			session := podmanTest.Quadlet([]string{"-dryrun"}, quadletDir)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))

			current := session.OutputToStringArray()
			expected := []string{
				"---basic.service---",
				"## assert-podman-args \"kube\"",
				"## assert-podman-args \"play\"",
				"## assert-podman-final-args-regex /tmp/podman_test.*/quadlet/deployment.yml",
				"## assert-podman-args \"--replace\"",
				"## assert-podman-args \"--service-container=true\"",
				"## assert-podman-stop-args \"kube\"",
				"## assert-podman-stop-args \"down\"",
				"## assert-podman-stop-final-args-regex /tmp/podman_test.*/quadlet/deployment.yml",
				"## assert-key-is \"Unit\" \"RequiresMountsFor\" \"%t/containers\"",
				"## assert-key-is \"Service\" \"KillMode\" \"mixed\"",
				"## assert-key-is \"Service\" \"Type\" \"notify\"",
				"## assert-key-is \"Service\" \"NotifyAccess\" \"all\"",
				"## assert-key-is \"Service\" \"Environment\" \"PODMAN_SYSTEMD_UNIT=%n\"",
				"## assert-key-is \"Service\" \"SyslogIdentifier\" \"%N\"",
				"[X-Kube]",
				"Yaml=deployment.yml",
				"[Unit]",
				fmt.Sprintf("SourcePath=%s/basic.kube", quadletDir),
				"RequiresMountsFor=%t/containers",
				"[Service]",
				"KillMode=mixed",
				"Environment=PODMAN_SYSTEMD_UNIT=%n",
				"Type=notify",
				"NotifyAccess=all",
				"SyslogIdentifier=%N",
				fmt.Sprintf("ExecStart=/usr/local/bin/podman kube play --replace --service-container=true %s/deployment.yml", quadletDir),
				fmt.Sprintf("ExecStop=/usr/local/bin/podman kube down %s/deployment.yml", quadletDir),
			}

			Expect(expected).To(Equal(current))
		})
	})

	DescribeTable("Running quadlet test case",
		func(fileName string) {
			testcase := loadQuadletTestcase(filepath.Join("quadlet", fileName))

			// Write the tested file to the quadlet dir
			err = os.WriteFile(filepath.Join(quadletDir, fileName), testcase.data, 0644)
			Expect(err).ToNot(HaveOccurred())

			// Run quadlet to convert the file
			session := podmanTest.Quadlet([]string{"-no-kmsg-log", generatedDir}, quadletDir)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))

			// Print any stderr output
			errs := session.ErrorToString()
			if errs != "" {
				fmt.Println("error:", session.ErrorToString())
			}

			testcase.check(generatedDir, session)
		},
		Entry("Basic container", "basic.container"),
		Entry("annotation.container", "annotation.container"),
		Entry("basepodman.container", "basepodman.container"),
		Entry("capabilities.container", "capabilities.container"),
		Entry("capabilities2.container", "capabilities2.container"),
		Entry("devices.container", "devices.container"),
		Entry("env.container", "env.container"),
		Entry("escapes.container", "escapes.container"),
		Entry("exec.container", "exec.container"),
		Entry("image.container", "image.container"),
		Entry("install.container", "install.container"),
		Entry("label.container", "label.container"),
		Entry("name.container", "name.container"),
		Entry("network.container", "network.container"),
		Entry("network.quadlet.container", "network.quadlet.container"),
		Entry("noimage.container", "noimage.container"),
		Entry("notify.container", "notify.container"),
		Entry("oneshot.container", "oneshot.container"),
		Entry("other-sections.container", "other-sections.container"),
		Entry("podmanargs.container", "podmanargs.container"),
		Entry("ports.container", "ports.container"),
		Entry("ports_ipv6.container", "ports_ipv6.container"),
		Entry("readonly-notmpfs.container", "readonly-notmpfs.container"),
		Entry("readwrite.container", "readwrite.container"),
		Entry("readwrite-notmpfs.container", "readwrite-notmpfs.container"),
		Entry("seccomp.container", "seccomp.container"),
		Entry("shortname.container", "shortname.container"),
		Entry("timezone.container", "timezone.container"),
		Entry("user.container", "user.container"),
		Entry("remap-manual.container", "remap-manual.container"),
		Entry("remap-auto.container", "remap-auto.container"),
		Entry("remap-auto2.container", "remap-auto2.container"),
		Entry("volume.container", "volume.container"),
		Entry("env-file.container", "env-file.container"),
		Entry("env-host.container", "env-host.container"),
		Entry("env-host-false.container", "env-host-false.container"),

		Entry("basic.volume", "basic.volume"),
		Entry("label.volume", "label.volume"),
		Entry("uid.volume", "uid.volume"),
		Entry("device.volume", "device-copy.volume"),

		Entry("Basic kube", "basic.kube"),
		Entry("Syslog Identifier", "syslog.identifier.kube"),
		Entry("Absolute Path", "absolute.path.kube"),
		Entry("Kube - User Remap Manual", "remap-manual.kube"),
		Entry("Kube - User Remap Auto", "remap-auto.kube"),
		Entry("Kube - User Remap Auto with IDs", "remap-auto2.kube"),
		Entry("Kube - Network", "network.kube"),
		Entry("Kube - Quadlet Network", "network.quadlet.kube"),
		Entry("Kube - ConfigMap", "configmap.kube"),
		Entry("Kube - Publish IPv4 ports", "ports.kube"),
		Entry("Kube - Publish IPv6 ports", "ports_ipv6.kube"),

		Entry("Network - Basic", "basic.network"),
		Entry("Network - Label", "label.network"),
		Entry("Network - Disable DNS", "disable-dns.network"),
		Entry("Network - Driver", "driver.network"),
		Entry("Network - Subnets", "subnets.network"),
		Entry("Network - Gateway", "gateway.network"),
		Entry("Network - Gateway without Subnet", "gateway.no-subnet.network"),
		Entry("Network - Gateway not enough Subnet", "gateway.less-subnet.network"),
		Entry("Network - Range", "range.network"),
		Entry("Network - Range without Subnet", "range.no-subnet.network"),
		Entry("Network - Range not enough Subnet", "range.less-subnet.network"),
		Entry("Network - subnet, gateway and range", "subnet-trio.network"),
		Entry("Network - multiple subnet, gateway and range", "subnet-trio.multiple.network"),
		Entry("Network - Internal network", "internal.network"),
		Entry("Network - IPAM Driver", "ipam-driver.network"),
		Entry("Network - IPv6", "ipv6.network"),
		Entry("Network - Options", "options.network"),
		Entry("Network - Multiple Options", "options.multiple.network"),
	)

})
