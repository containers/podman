package e2e

import (
	"encoding/json"

	"github.com/containers/podman/v4/pkg/machine/qemu"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("inspect two machines", func() {
		i := new(initMachine)
		foo1, err := mb.setName("foo1").setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).To(BeNil())
		Expect(foo1.ExitCode()).To(Equal(0))

		ii := new(initMachine)
		foo2, err := mb.setName("foo2").setCmd(ii.withImagePath(mb.imagePath)).run()
		Expect(err).To(BeNil())
		Expect(foo2.ExitCode()).To(Equal(0))

		inspect := new(inspectMachine)
		inspectSession, err := mb.setName("foo1").setCmd(inspect).run()
		Expect(err).To(BeNil())
		Expect(inspectSession.ExitCode()).To(Equal(0))

		type fakeInfos struct {
			Status string
			VM     qemu.MachineVM
		}
		infos := make([]fakeInfos, 0, 2)
		err = json.Unmarshal(inspectSession.Bytes(), &infos)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(infos)).To(Equal(2))

		//rm := new(rmMachine)
		////	Must manually clean up due to multiple names
		//for _, name := range []string{"foo1", "foo2"} {
		//	mb.setName(name).setCmd(rm.withForce()).run()
		//	mb.names = []string{}
		//}
		//mb.names = []string{}

	})
})
