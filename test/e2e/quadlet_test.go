package integration

import (
	"fmt"
	"os"
	"path/filepath"
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
	Expect(err).To(BeNil())

	base := filepath.Base(path)
	ext := filepath.Ext(base)
	service := base[:len(base)-len(ext)]
	if ext == ".volume" {
		service += "-volume"
	}
	service += ".service"

	checks := make([][]string, 0)

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "##") {
			words, err := shellwords.Parse(line[2:])
			Expect(err).To(BeNil())
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

func (t *quadletTestcase) assertStdErrContains(args []string, session *PodmanSessionIntegration) bool {
	return strings.Contains(session.OutputToString(), args[0])
}

func (t *quadletTestcase) assertKeyIs(args []string, unit *parser.UnitFile) bool {
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

func (t *quadletTestcase) assertKeyContains(args []string, unit *parser.UnitFile) bool {
	group := args[0]
	key := args[1]
	value := args[2]

	realValue, ok := unit.LookupLast(group, key)
	return ok && strings.Contains(realValue, value)
}

func (t *quadletTestcase) assertPodmanArgs(args []string, unit *parser.UnitFile) bool {
	podmanArgs, _ := unit.LookupLastArgs("Service", "ExecStart")
	return findSublist(podmanArgs, args) != -1
}

func (t *quadletTestcase) assertFinalArgs(args []string, unit *parser.UnitFile) bool {
	podmanArgs, _ := unit.LookupLastArgs("Service", "ExecStart")
	if len(podmanArgs) < len(args) {
		return false
	}
	return matchSublistAt(podmanArgs, len(podmanArgs)-len(args), args)
}

func (t *quadletTestcase) assertSymlink(args []string, unit *parser.UnitFile) bool {
	symlink := args[0]
	expectedTarget := args[1]

	dir := filepath.Dir(unit.Path)

	target, err := os.Readlink(filepath.Join(dir, symlink))
	Expect(err).ToNot(HaveOccurred())

	return expectedTarget == target
}

func (t *quadletTestcase) doAssert(check []string, unit *parser.UnitFile, session *PodmanSessionIntegration) error {
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
	case "assert-key-contains":
		ok = t.assertKeyContains(args, unit)
	case "assert-podman-args":
		ok = t.assertPodmanArgs(args, unit)
	case "assert-podman-final-args":
		ok = t.assertFinalArgs(args, unit)
	case "assert-symlink":
		ok = t.assertSymlink(args, unit)
	default:
		return fmt.Errorf("Unsupported assertion %s", op)
	}
	if invert {
		ok = !ok
	}

	if !ok {
		s, _ := unit.ToString()
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
	if _, err := os.Stat(file); os.IsNotExist(err) && expectFail {
		return // Successful fail
	}

	unit, err := parser.ParseUnitFile(file)
	Expect(err).To(BeNil())

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
		Expect(err).To(BeNil())

		quadletDir = filepath.Join(podmanTest.TempDir, "quadlet")
		err = os.Mkdir(quadletDir, os.ModePerm)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	DescribeTable("Running quadlet test case",
		func(fileName string) {
			testcase := loadQuadletTestcase(filepath.Join("quadlet", fileName))

			// Write the tested file to the quadlet dir
			err = os.WriteFile(filepath.Join(quadletDir, fileName), testcase.data, 0644)
			Expect(err).To(BeNil())

			// Run quadlet to convert the file
			session := podmanTest.Quadlet([]string{generatedDir}, quadletDir)
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
		Entry("env.container", "env.container"),
		Entry("escapes.container", "escapes.container"),
		Entry("exec.container", "exec.container"),
		Entry("image.container", "image.container"),
		Entry("install.container", "install.container"),
		Entry("label.container", "label.container"),
		Entry("name.container", "name.container"),
		Entry("noimage.container", "noimage.container"),
		Entry("noremapuser2.container", "noremapuser2.container"),
		Entry("noremapuser.container", "noremapuser.container"),
		Entry("notify.container", "notify.container"),
		Entry("other-sections.container", "other-sections.container"),
		Entry("podmanargs.container", "podmanargs.container"),
		Entry("ports.container", "ports.container"),
		Entry("ports_ipv6.container", "ports_ipv6.container"),
		Entry("socketactivated.container", "socketactivated.container"),
		Entry("timezone.container", "timezone.container"),
		Entry("user.container", "user.container"),
		Entry("user-host.container", "user-host.container"),
		Entry("user-root1.container", "user-root1.container"),
		Entry("user-root2.container", "user-root2.container"),
		Entry("volume.container", "volume.container"),

		Entry("basic.volume", "basic.volume"),
		Entry("label.volume", "label.volume"),
		Entry("uid.volume", "uid.volume"),
	)

})
