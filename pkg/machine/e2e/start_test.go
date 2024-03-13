package e2e_test

import (
	"sync"
	"time"

	"github.com/containers/podman/v5/pkg/machine/define"
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
		session, err := mb.setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))
		s := new(startMachine)
		startSession, err := mb.setCmd(s).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))

		info, ec, err := mb.toQemuInspectInfo()
		Expect(err).ToNot(HaveOccurred())
		Expect(ec).To(BeZero())
		Expect(info[0].State).To(Equal(define.Running))

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
		session, err := mb.setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))
		s := new(startMachine)
		startSession, err := mb.setCmd(s).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))

		info, ec, err := mb.toQemuInspectInfo()
		Expect(err).ToNot(HaveOccurred())
		Expect(ec).To(BeZero())
		Expect(info[0].State).To(Equal(define.Running))

		startSession, err = mb.setCmd(s).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(125))
		Expect(startSession.errorToString()).To(ContainSubstring("VM already running or starting"))
	})

	It("start only starts specified machine", func() {
		i := initMachine{}
		startme := randomString()
		session, err := mb.setName(startme).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		j := initMachine{}
		dontstartme := randomString()
		session2, err := mb.setName(dontstartme).setCmd(j.withImage(mb.imagePath)).run()
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
		Expect(inspectSession.outputToString()).To(Equal(define.Running))

		inspect2 := new(inspectMachine)
		inspect2 = inspect2.withFormat("{{.State}}")
		inspectSession2, err := mb.setName(dontstartme).setCmd(inspect2).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession2).To(Exit(0))
		Expect(inspectSession2.outputToString()).To(Not(Equal(define.Running)))
	})

	It("start two machines in parallel", func() {
		i := initMachine{}
		machine1 := "m1-" + randomString()
		session, err := mb.setName(machine1).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		machine2 := "m2-" + randomString()
		session, err = mb.setName(machine2).setCmd(i.withImage(mb.imagePath)).run()
		Expect(session).To(Exit(0))

		var startSession1, startSession2 *machineSession
		wg := sync.WaitGroup{}
		wg.Add(2)
		// now start two machine start process in parallel
		go func() {
			defer GinkgoRecover()
			defer wg.Done()
			s := startMachine{}
			startSession1, err = mb.setName(machine1).setCmd(s).setTimeout(time.Minute * 10).run()
			Expect(err).ToNot(HaveOccurred())
		}()
		go func() {
			defer GinkgoRecover()
			defer wg.Done()
			s := startMachine{}
			// ok this is a hack and should not be needed but the way these test are setup they all
			// share "mb" which stores the name that is used for the VM, thus running two parallel
			// can overwrite the name from the other, work around that by creating a new mb for the
			// second run.
			nmb, err := newMB()
			Expect(err).ToNot(HaveOccurred())
			startSession2, err = nmb.setName(machine2).setCmd(s).setTimeout(time.Minute * 10).run()
			Expect(err).ToNot(HaveOccurred())
		}()
		wg.Wait()

		// WSL can start in parallel so just check both command exit 0 there
		if testProvider.VMType() == define.WSLVirt {
			Expect(startSession1).To(Exit(0))
			Expect(startSession2).To(Exit(0))
			return
		}
		// other providers have a check that only one VM can be running at any given time so make sure our check is race free
		Expect(startSession1).To(Or(Exit(0), Exit(125)), "start command should succeed or fail with 125")
		if startSession1.ExitCode() == 0 {
			Expect(startSession2).To(Exit(125), "first start worked, second start must fail")
			Expect(startSession2.errorToString()).To(ContainSubstring("machine %s: VM already running or starting", machine1))
		} else {
			Expect(startSession2).To(Exit(0), "first start failed, second start succeed")
			Expect(startSession1.errorToString()).To(ContainSubstring("machine %s: VM already running or starting", machine2))
		}
	})
})
