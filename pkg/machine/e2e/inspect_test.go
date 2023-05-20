package e2e_test

import (
	"strings"

	"github.com/containers/podman/v4/pkg/machine"
	jsoniter "github.com/json-iterator/go"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine stop", func() {
	var (
		mb      *machineTestBuilder
		testDir string
	)

	BeforeEach(func() {
		testDir, mb = setup()
	})
	AfterEach(func() {
		teardown(originalHomeDir, testDir, mb)
	})

	It("inspect bad name", func() {
		i := inspectMachine{}
		reallyLongName := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		session, err := mb.setName(reallyLongName).setCmd(&i).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
	})

	It("inspect two machines", func() {
		i := new(initMachine)
		foo1, err := mb.setName("foo1").setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(foo1).To(Exit(0))

		ii := new(initMachine)
		foo2, err := mb.setName("foo2").setCmd(ii.withImagePath(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(foo2).To(Exit(0))

		inspect := new(inspectMachine)
		inspect = inspect.withFormat("{{.Name}}")
		inspectSession, err := mb.setName("foo1").setCmd(inspect).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.Bytes()).To(ContainSubstring("foo1"))
	})

	It("inspect with go format", func() {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// regular inspect should
		inspectJSON := new(inspectMachine)
		inspectSession, err := mb.setName(name).setCmd(inspectJSON).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))

		var inspectInfo []machine.InspectInfo
		err = jsoniter.Unmarshal(inspectSession.Bytes(), &inspectInfo)
		Expect(err).ToNot(HaveOccurred())
		Expect(strings.HasSuffix(inspectInfo[0].ConnectionInfo.PodmanSocket.GetPath(), "podman.sock"))

		inspect := new(inspectMachine)
		inspect = inspect.withFormat("{{.Name}}")
		inspectSession, err = mb.setName(name).setCmd(inspect).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.Bytes()).To(ContainSubstring(name))

		// check invalid template returns error
		inspect = new(inspectMachine)
		inspect = inspect.withFormat("{{.Abcde}}")
		inspectSession, err = mb.setName(name).setCmd(inspect).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(125))
		Expect(inspectSession.errorToString()).To(ContainSubstring("can't evaluate field Abcde in type machine.InspectInfo"))
	})
})
