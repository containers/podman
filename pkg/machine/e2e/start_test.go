package e2e_test

import (
	"time"

	"github.com/containers/podman/v4/pkg/machine"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine start", func() {
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

	It("start simple machine", func() {
		i := new(initMachine)
		session, err := mb.setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))
		s := new(startMachine)
		startSession, err := mb.setCmd(s).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))

		info, ec, err := mb.toQemuInspectInfo()
		Expect(err).ToNot(HaveOccurred())
		Expect(ec).To(BeZero())
		Expect(info[0].State).To(Equal(machine.Running))

		stop := new(stopMachine)
		stopSession, err := mb.setCmd(stop).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopSession).To(Exit(0))

		// suppress output
		startSession, err = mb.setCmd(s.withNoInfo()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))
		Expect(startSession.outputToString()).ToNot(ContainSubstring("API forwarding"))

		stopSession, err = mb.setCmd(stop).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopSession).To(Exit(0))

		startSession, err = mb.setCmd(s.withQuiet()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))
		Expect(startSession.outputToStringSlice()).To(HaveLen(1))
	})

	It("bad start name", func() {
		i := startMachine{}
		reallyLongName := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		session, err := mb.setName(reallyLongName).setCmd(&i).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
		Expect(session.errorToString()).To(ContainSubstring("VM does not exist"))
	})

	It("start machine already started", func() {
		i := new(initMachine)
		session, err := mb.setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))
		s := new(startMachine)
		startSession, err := mb.setCmd(s).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))

		info, ec, err := mb.toQemuInspectInfo()
		Expect(err).ToNot(HaveOccurred())
		Expect(ec).To(BeZero())
		Expect(info[0].State).To(Equal(machine.Running))

		startSession, err = mb.setCmd(s).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(125))
		Expect(startSession.errorToString()).To(ContainSubstring("VM already running or starting"))
	})
	It("start only starts specified machine", func() {
		i := initMachine{}
		startme := randomString()
		session, err := mb.setName(startme).setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		j := initMachine{}
		dontstartme := randomString()
		session2, err := mb.setName(dontstartme).setCmd(j.withImagePath(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session2).To(Exit(0))

		s := startMachine{}
		session3, err := mb.setName(startme).setCmd(s).setTimeout(time.Minute * 10).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session3).Should(Exit(0))

		inspect := new(inspectMachine)
		inspect = inspect.withFormat("{{.State}}")
		inspectSession, err := mb.setName(startme).setCmd(inspect).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.outputToString()).To(Equal(machine.Running))

		inspect2 := new(inspectMachine)
		inspect2 = inspect2.withFormat("{{.State}}")
		inspectSession2, err := mb.setName(dontstartme).setCmd(inspect2).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession2).To(Exit(0))
		Expect(inspectSession2.outputToString()).To(Not(Equal(machine.Running)))
	})
})
