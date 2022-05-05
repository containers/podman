package e2e

import (
	"encoding/json"

	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/qemu"
	jsoniter "github.com/json-iterator/go"

	. "github.com/onsi/ginkgo"
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
		Expect(err).To(BeNil())
		Expect(session).To(Exit(125))
	})

	It("inspect two machines", func() {
		i := new(initMachine)
		foo1, err := mb.setName("foo1").setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).To(BeNil())
		Expect(foo1).To(Exit(0))

		ii := new(initMachine)
		foo2, err := mb.setName("foo2").setCmd(ii.withImagePath(mb.imagePath)).run()
		Expect(err).To(BeNil())
		Expect(foo2).To(Exit(0))

		inspect := new(inspectMachine)
		inspect = inspect.withFormat("{{.Name}}")
		inspectSession, err := mb.setName("foo1").setCmd(inspect).run()
		Expect(err).To(BeNil())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.Bytes()).To(ContainSubstring("foo1"))

		type fakeInfos struct {
			Status string
			VM     qemu.MachineVM
		}
		infos := make([]fakeInfos, 0, 2)
		err = json.Unmarshal(inspectSession.Bytes(), &infos)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(infos)).To(Equal(2))

		// rm := new(rmMachine)
		// //	Must manually clean up due to multiple names
		// for _, name := range []string{"foo1", "foo2"} {
		//	mb.setName(name).setCmd(rm.withForce()).run()
		//	mb.names = []string{}
		// }
		// mb.names = []string{}

	})

	It("inspect with go format", func() {
		name := randomString(12)
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).To(BeNil())
		Expect(session).To(Exit(0))

		// regular inspect should
		inspectJson := new(inspectMachine)
		inspectSession, err := mb.setName(name).setCmd(inspectJson).run()
		Expect(err).To(BeNil())
		Expect(inspectSession).To(Exit(0))

		var inspectInfo []machine.InspectInfo
		err = jsoniter.Unmarshal(inspectSession.Bytes(), &inspectInfo)
		Expect(err).To(BeNil())

		inspect := new(inspectMachine)
		inspect = inspect.withFormat("{{.Name}}")
		inspectSession, err = mb.setName(name).setCmd(inspect).run()
		Expect(err).To(BeNil())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.Bytes()).To(ContainSubstring(name))
	})
})
