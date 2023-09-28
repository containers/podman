package integration

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/containers/podman/v4/pkg/systemd/parser"
	. "github.com/containers/podman/v4/test/utils"
	"github.com/containers/podman/v4/version"
	"github.com/mattn/go-shellwords"

	. "github.com/onsi/ginkgo/v2"
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
	case ".image":
		service += "-image"
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

func keyValueStringToMap(keyValueString, separator string) (map[string]string, error) {
	keyValMap := make(map[string]string)
	csvReader := csv.NewReader(strings.NewReader(keyValueString))
	csvReader.Comma = []rune(separator)[0]
	keyVarList, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}
	for _, param := range keyVarList[0] {
		val := ""
		kv := strings.SplitN(param, "=", 2)
		if len(kv) == 2 {
			val = kv[1]
		}
		keyValMap[kv[0]] = val
	}

	return keyValMap, nil
}

func keyValMapEqualRegex(expectedKeyValMap, actualKeyValMap map[string]string) bool {
	if len(expectedKeyValMap) != len(actualKeyValMap) {
		return false
	}
	for key, expectedValue := range expectedKeyValMap {
		actualValue, ok := actualKeyValMap[key]
		if !ok {
			return false
		}
		matched, err := regexp.MatchString(expectedValue, actualValue)
		if err != nil || !matched {
			return false
		}
	}
	return true
}

func (t *quadletTestcase) assertPodmanArgsKeyVal(args []string, unit *parser.UnitFile, key string, allowRegex bool) bool {
	podmanArgs, _ := unit.LookupLastArgs("Service", key)

	expectedKeyValMap, err := keyValueStringToMap(args[2], args[1])
	if err != nil {
		return false
	}
	argKeyLocation := 0
	for {
		subListLocation := findSublist(podmanArgs[argKeyLocation:], []string{args[0]})
		if subListLocation == -1 {
			break
		}

		argKeyLocation += subListLocation
		actualKeyValMap, err := keyValueStringToMap(podmanArgs[argKeyLocation+1], args[1])
		if err != nil {
			break
		}
		if allowRegex {
			if keyValMapEqualRegex(expectedKeyValMap, actualKeyValMap) {
				return true
			}
		} else if reflect.DeepEqual(expectedKeyValMap, actualKeyValMap) {
			return true
		}

		argKeyLocation += 2

		if argKeyLocation > len(podmanArgs) {
			break
		}
	}

	return false
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

func (t *quadletTestcase) assertStartPodmanArgsKeyVal(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgsKeyVal(args, unit, "ExecStart", false)
}

func (t *quadletTestcase) assertStartPodmanArgsKeyValRegex(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgsKeyVal(args, unit, "ExecStart", true)
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

func (t *quadletTestcase) assertStopPostPodmanArgs(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgs(args, unit, "ExecStopPost")
}

func (t *quadletTestcase) assertStopPostPodmanFinalArgs(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanFinalArgs(args, unit, "ExecStopPost")
}

func (t *quadletTestcase) assertStopPostPodmanFinalArgsRegex(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanFinalArgsRegex(args, unit, "ExecStopPost")
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
	case "assert-podman-args-key-val":
		ok = t.assertStartPodmanArgsKeyVal(args, unit)
	case "assert-podman-args-key-val-regex":
		ok = t.assertStartPodmanArgsKeyValRegex(args, unit)
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
	case "assert-podman-stop-post-args":
		ok = t.assertStopPostPodmanArgs(args, unit)
	case "assert-podman-stop-post-final-args":
		ok = t.assertStopPostPodmanFinalArgs(args, unit)
	case "assert-podman-stop-post-final-args-regex":
		ok = t.assertStopPostPodmanFinalArgsRegex(args, unit)

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
		err          error
		generatedDir string
		quadletDir   string
	)

	BeforeEach(func() {
		generatedDir = filepath.Join(podmanTest.TempDir, "generated")
		err = os.Mkdir(generatedDir, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		quadletDir = filepath.Join(podmanTest.TempDir, "quadlet")
		err = os.Mkdir(quadletDir, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("quadlet -version", func() {
		It("Should print correct version", func() {
			session := podmanTest.Quadlet([]string{"-version"}, "/something")
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal(version.Version.String()))
		})
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
			Expect(session).Should(Exit(0))

			current := session.ErrorToStringArray()
			expected := "No files parsed from [/something]"

			Expect(current[0]).To(ContainSubstring(expected))
		})

		It("Should fail on bad quadlet", func() {
			quadletfile := fmt.Sprintf(`[Container]
Image=%s
BOGUS=foo
`, ALPINE)

			quadletfilePath := filepath.Join(podmanTest.TempDir, "bogus.container")
			err = os.WriteFile(quadletfilePath, []byte(quadletfile), 0644)
			Expect(err).ToNot(HaveOccurred())
			defer os.Remove(quadletfilePath)
			session := podmanTest.Quadlet([]string{"-dryrun"}, podmanTest.TempDir)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(1))
		})

		It("Should scan and return output for files in subdirectories", func() {
			dirName := "test_subdir"

			err = CopyDirectory(filepath.Join("quadlet", dirName), quadletDir)

			if err != nil {
				GinkgoWriter.Println("error:", err)
			}

			session := podmanTest.Quadlet([]string{"-dryrun", "-user"}, quadletDir)
			session.WaitWithDefaultTimeout()

			current := session.OutputToStringArray()
			expected := []string{
				"---mysleep.service---",
				"---mysleep_1.service---",
				"---mysleep_2.service---",
			}

			Expect(current).To(ContainElements(expected))
		})

		It("Should parse a kube file and print it to stdout", func() {
			fileName := "basic.kube"
			testcase := loadQuadletTestcase(filepath.Join("quadlet", fileName))

			// quadlet uses PODMAN env to get a stable podman path
			podmanPath, found := os.LookupEnv("PODMAN")
			if !found {
				podmanPath = podmanTest.PodmanBinary
			}

			// Write the tested file to the quadlet dir
			err = os.WriteFile(filepath.Join(quadletDir, fileName), testcase.data, 0644)
			Expect(err).ToNot(HaveOccurred())

			session := podmanTest.Quadlet([]string{"-dryrun"}, quadletDir)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.ErrorToString()).To(ContainSubstring("Loading source unit file "))

			current := session.OutputToStringArray()
			expected := []string{
				"---basic.service---",
				"## assert-podman-args \"kube\"",
				"## assert-podman-args \"play\"",
				"## assert-podman-final-args-regex .*/podman_test.*/quadlet/deployment.yml",
				"## assert-podman-args \"--replace\"",
				"## assert-podman-args \"--service-container=true\"",
				"## assert-podman-stop-post-args \"kube\"",
				"## assert-podman-stop-post-args \"down\"",
				"## assert-podman-stop-post-final-args-regex .*/podman_test.*/quadlet/deployment.yml",
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
				fmt.Sprintf("ExecStart=%s kube play --replace --service-container=true %s/deployment.yml", podmanPath, quadletDir),
				fmt.Sprintf("ExecStopPost=%s kube down %s/deployment.yml", podmanPath, quadletDir),
			}

			Expect(current).To(Equal(expected))
		})
	})

	DescribeTable("Running quadlet test case",
		func(fileName string, exitCode int, errString string) {
			testcase := loadQuadletTestcase(filepath.Join("quadlet", fileName))

			// Write the tested file to the quadlet dir
			err = os.WriteFile(filepath.Join(quadletDir, fileName), testcase.data, 0644)
			Expect(err).ToNot(HaveOccurred())

			// Run quadlet to convert the file
			session := podmanTest.Quadlet([]string{"--user", "--no-kmsg-log", generatedDir}, quadletDir)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(exitCode))

			// Print any stderr output
			errs := session.ErrorToString()
			if errs != "" {
				GinkgoWriter.Println("error:", session.ErrorToString())
			}
			Expect(errs).Should(ContainSubstring(errString))

			testcase.check(generatedDir, session)
		},
		Entry("Basic container", "basic.container", 0, ""),
		Entry("annotation.container", "annotation.container", 0, ""),
		Entry("autoupdate.container", "autoupdate.container", 0, ""),
		Entry("basepodman.container", "basepodman.container", 0, ""),
		Entry("capabilities.container", "capabilities.container", 0, ""),
		Entry("capabilities2.container", "capabilities2.container", 0, ""),
		Entry("devices.container", "devices.container", 0, ""),
		Entry("disableselinux.container", "disableselinux.container", 0, ""),
		Entry("dns-options.container", "dns-options.container", 0, ""),
		Entry("dns-search.container", "dns-search.container", 0, ""),
		Entry("dns.container", "dns.container", 0, ""),
		Entry("env-file.container", "env-file.container", 0, ""),
		Entry("env-host-false.container", "env-host-false.container", 0, ""),
		Entry("env-host.container", "env-host.container", 0, ""),
		Entry("env.container", "env.container", 0, ""),
		Entry("escapes.container", "escapes.container", 0, ""),
		Entry("exec.container", "exec.container", 0, ""),
		Entry("health.container", "health.container", 0, ""),
		Entry("hostname.container", "hostname.container", 0, ""),
		Entry("image.container", "image.container", 0, ""),
		Entry("install.container", "install.container", 0, ""),
		Entry("ip.container", "ip.container", 0, ""),
		Entry("label.container", "label.container", 0, ""),
		Entry("logdriver.container", "logdriver.container", 0, ""),
		Entry("mask.container", "mask.container", 0, ""),
		Entry("mount.container", "mount.container", 0, ""),
		Entry("name.container", "name.container", 0, ""),
		Entry("nestedselinux.container", "nestedselinux.container", 0, ""),
		Entry("network.container", "network.container", 0, ""),
		Entry("network.quadlet.container", "network.quadlet.container", 0, ""),
		Entry("noimage.container", "noimage.container", 1, "converting \"noimage.container\": no Image or Rootfs key specified"),
		Entry("notify.container", "notify.container", 0, ""),
		Entry("oneshot.container", "oneshot.container", 0, ""),
		Entry("other-sections.container", "other-sections.container", 0, ""),
		Entry("podmanargs.container", "podmanargs.container", 0, ""),
		Entry("ports.container", "ports.container", 0, ""),
		Entry("ports_ipv6.container", "ports_ipv6.container", 0, ""),
		Entry("pull.container", "pull.container", 0, ""),
		Entry("readonly-notmpfs.container", "readonly-notmpfs.container", 0, ""),
		Entry("readwrite-notmpfs.container", "readwrite-notmpfs.container", 0, ""),
		Entry("readwrite.container", "readwrite.container", 0, ""),
		Entry("remap-auto.container", "remap-auto.container", 0, ""),
		Entry("remap-auto2.container", "remap-auto2.container", 0, ""),
		Entry("remap-keep-id.container", "remap-keep-id.container", 0, ""),
		Entry("remap-keep-id2.container", "remap-keep-id2.container", 0, ""),
		Entry("remap-manual.container", "remap-manual.container", 0, ""),
		Entry("rootfs.container", "rootfs.container", 0, ""),
		Entry("seccomp.container", "seccomp.container", 0, ""),
		Entry("secrets.container", "secrets.container", 0, ""),
		Entry("selinux.container", "selinux.container", 0, ""),
		Entry("shmsize.container", "shmsize.container", 0, ""),
		Entry("shortname.container", "shortname.container", 0, "Warning: shortname.container specifies the image \"shortname\" which not a fully qualified image name. This is not ideal for performance and security reasons. See the podman-pull manpage discussion of short-name-aliases.conf for details."),
		Entry("sysctl.container", "sysctl.container", 0, ""),
		Entry("timezone.container", "timezone.container", 0, ""),
		Entry("unmask.container", "unmask.container", 0, ""),
		Entry("user.container", "user.container", 0, ""),
		Entry("volume.container", "volume.container", 0, ""),
		Entry("workingdir.container", "workingdir.container", 0, ""),

		Entry("basic.volume", "basic.volume", 0, ""),
		Entry("device-copy.volume", "device-copy.volume", 0, ""),
		Entry("device.volume", "device.volume", 0, ""),
		Entry("label.volume", "label.volume", 0, ""),
		Entry("name.volume", "name.volume", 0, ""),
		Entry("podmanargs.volume", "podmanargs.volume", 0, ""),
		Entry("uid.volume", "uid.volume", 0, ""),
		Entry("image.volume", "image.volume", 0, ""),
		Entry("image-no-image.volume", "image-no-image.volume", 1, "converting \"image-no-image.volume\": the key Image is mandatory when using the image driver"),

		Entry("Absolute Path", "absolute.path.kube", 0, ""),
		Entry("Basic kube", "basic.kube", 0, ""),
		Entry("Kube - ConfigMap", "configmap.kube", 0, ""),
		Entry("Kube - Exit Code Propagation", "exit_code_propagation.kube", 0, ""),
		Entry("Kube - Logdriver", "logdriver.kube", 0, ""),
		Entry("Kube - Network", "network.kube", 0, ""),
		Entry("Kube - PodmanArgs", "podmanargs.kube", 0, ""),
		Entry("Kube - Publish IPv4 ports", "ports.kube", 0, ""),
		Entry("Kube - Publish IPv6 ports", "ports_ipv6.kube", 0, ""),
		Entry("Kube - Quadlet Network", "network.quadlet.kube", 0, ""),
		Entry("Kube - User Remap Auto with IDs", "remap-auto2.kube", 0, ""),
		Entry("Kube - User Remap Auto", "remap-auto.kube", 0, ""),
		Entry("Kube - User Remap Manual", "remap-manual.kube", 1, "converting \"remap-manual.kube\": RemapUsers=manual is not supported"),
		Entry("Syslog Identifier", "syslog.identifier.kube", 0, ""),
		Entry("Kube - Working Directory YAML Absolute Path", "workingdir-yaml-abs.kube", 0, ""),
		Entry("Kube - Working Directory YAML Relative Path", "workingdir-yaml-rel.kube", 0, ""),
		Entry("Kube - Working Directory Unit", "workingdir-unit.kube", 0, ""),
		Entry("Kube - Working Directory already in Service", "workingdir-service.kube", 0, ""),

		Entry("Network - Basic", "basic.network", 0, ""),
		Entry("Network - Disable DNS", "disable-dns.network", 0, ""),
		Entry("Network - DNS", "dns.network", 0, ""),
		Entry("Network - Driver", "driver.network", 0, ""),
		Entry("Network - Gateway not enough Subnet", "gateway.less-subnet.network", 1, "converting \"gateway.less-subnet.network\": cannot set more gateways than subnets"),
		Entry("Network - Gateway without Subnet", "gateway.no-subnet.network", 1, "converting \"gateway.no-subnet.network\": cannot set gateway or range without subnet"),
		Entry("Network - Gateway", "gateway.network", 0, ""),
		Entry("Network - IPAM Driver", "ipam-driver.network", 0, ""),
		Entry("Network - IPv6", "ipv6.network", 0, ""),
		Entry("Network - Internal network", "internal.network", 0, ""),
		Entry("Network - Label", "label.network", 0, ""),
		Entry("Network - Multiple Options", "options.multiple.network", 0, ""),
		Entry("Network - Name", "name.network", 0, ""),
		Entry("Network - Options", "options.network", 0, ""),
		Entry("Network - PodmanArgs", "podmanargs.network", 0, ""),
		Entry("Network - Range not enough Subnet", "range.less-subnet.network", 1, "converting \"range.less-subnet.network\": cannot set more ranges than subnets"),
		Entry("Network - Range without Subnet", "range.no-subnet.network", 1, "converting \"range.no-subnet.network\": cannot set gateway or range without subnet"),
		Entry("Network - Range", "range.network", 0, ""),
		Entry("Network - Subnets", "subnets.network", 0, ""),
		Entry("Network - multiple subnet, gateway and range", "subnet-trio.multiple.network", 0, ""),
		Entry("Network - subnet, gateway and range", "subnet-trio.network", 0, ""),

		Entry("Image - Basic", "basic.image", 0, ""),
		Entry("Image - No Image", "no-image.image", 1, "converting \"no-image.image\": no Image key specified"),
		Entry("Image - Architecture", "arch.image", 0, ""),
		Entry("Image - Auth File", "auth.image", 0, ""),
		Entry("Image - Certificates", "certs.image", 0, ""),
		Entry("Image - Credentials", "creds.image", 0, ""),
		Entry("Image - Decryption Key", "decrypt.image", 0, ""),
		Entry("Image - OS Key", "os.image", 0, ""),
		Entry("Image - Variant Key", "variant.image", 0, ""),
		Entry("Image - All Tags", "all-tags.image", 0, ""),
		Entry("Image - TLS Verify", "tls-verify.image", 0, ""),
		Entry("Image - Arch and OS", "arch-os.image", 0, ""),
	)

})
