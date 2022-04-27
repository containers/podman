package e2e

import (
	"time"

	"github.com/containers/podman/v4/pkg/machine"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine init", func() {
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

	It("bad init name", func() {
		i := initMachine{}
		reallyLongName := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		session, err := mb.setName(reallyLongName).setCmd(&i).run()
		Expect(err).To(BeNil())
		Expect(session).To(Exit(125))
	})
	It("simple init", func() {
		i := new(initMachine)
		session, err := mb.setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).To(BeNil())
		Expect(session).To(Exit(0))

		inspectBefore, ec, err := mb.toQemuInspectInfo()
		Expect(err).To(BeNil())
		Expect(ec).To(BeZero())

		Expect(len(inspectBefore)).To(BeNumerically(">", 0))
		testMachine := inspectBefore[0]
		Expect(testMachine.VM.Name).To(Equal(mb.names[0]))
		Expect(testMachine.VM.CPUs).To(Equal(uint64(1)))
		Expect(testMachine.VM.Memory).To(Equal(uint64(2048)))

	})

	It("simple init with start", func() {
		i := initMachine{}
		session, err := mb.setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).To(BeNil())
		Expect(session).To(Exit(0))

		inspectBefore, ec, err := mb.toQemuInspectInfo()
		Expect(ec).To(BeZero())
		Expect(len(inspectBefore)).To(BeNumerically(">", 0))
		Expect(err).To(BeNil())
		Expect(len(inspectBefore)).To(BeNumerically(">", 0))
		Expect(inspectBefore[0].VM.Name).To(Equal(mb.names[0]))

		s := startMachine{}
		ssession, err := mb.setCmd(s).setTimeout(time.Minute * 10).run()
		Expect(err).To(BeNil())
		Expect(ssession).Should(Exit(0))

		inspectAfter, ec, err := mb.toQemuInspectInfo()
		Expect(err).To(BeNil())
		Expect(ec).To(BeZero())
		Expect(len(inspectBefore)).To(BeNumerically(">", 0))
		Expect(len(inspectAfter)).To(BeNumerically(">", 0))
		Expect(inspectAfter[0].State).To(Equal(machine.Running))
	})

})
