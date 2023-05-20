package e2e_test

import (
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
})
