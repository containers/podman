package utils_test

import (
	"io"
	"os/exec"
	"strings"
	"testing"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var FakeOutputs map[string][]string
var GoechoPath = "../goecho/goecho"

type FakePodmanTest struct {
	PodmanTest
}

func FakePodmanTestCreate() *FakePodmanTest {
	FakeOutputs = make(map[string][]string)
	p := &FakePodmanTest{
		PodmanTest: PodmanTest{
			PodmanBinary: GoechoPath,
		},
	}

	p.PodmanMakeOptions = p.makeOptions
	return p
}

func (p *FakePodmanTest) makeOptions(args []string, noEvents, noCache bool) []string {
	return FakeOutputs[strings.Join(args, " ")]
}

func StartFakeCmdSession(args []string) *PodmanSession {
	var outWriter, errWriter io.Writer
	command := exec.Command(GoechoPath, args...)
	session, err := gexec.Start(command, outWriter, errWriter)
	if err != nil {
		GinkgoWriter.Println(err)
	}
	return &PodmanSession{session}
}

func TestUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Unit test for test utils package")
}
