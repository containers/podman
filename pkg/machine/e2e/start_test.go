package e2e_test

import (
	"fmt"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/containers/podman/v5/pkg/machine/define"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine start", func() {

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
		name := randomString()
		i := new(initMachine)
		machineTestBuilderInit := mb.setName(name).setCmd(i.withImage(mb.imagePath))
		session, err := machineTestBuilderInit.run()
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
		Expect(startSession.errorToString()).To(ContainSubstring(fmt.Sprintf("Error: unable to start %q: already running", machineTestBuilderInit.name)))
	})

	It("start machine with conflict on SSH port", func() {
		i := new(initMachine)
		session, err := mb.setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		inspect := new(inspectMachine)
		inspectSession, err := mb.setCmd(inspect.withFormat("{{.SSHConfig.Port}}")).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		inspectPort := inspectSession.outputToString()

		connections := new(listSystemConnection)
		connectionsSession, err := mb.setCmd(connections.withFormat("{{.URI}}")).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(connectionsSession).To(Exit(0))
		connectionURLs := connectionsSession.outputToStringSlice()
		connectionPorts, err := mapToPort(connectionURLs)
		Expect(err).ToNot(HaveOccurred())
		Expect(connectionPorts).To(HaveEach(inspectPort))

		// start a listener on the ssh port
		listener, err := net.Listen("tcp", "127.0.0.1:"+inspectPort)
		Expect(err).ToNot(HaveOccurred())
		defer listener.Close()

		s := new(startMachine)
		startSession, err := mb.setCmd(s).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))
		Expect(startSession.errorToString()).To(ContainSubstring("detected port conflict on machine ssh port"))

		inspect2 := new(inspectMachine)
		inspectSession2, err := mb.setCmd(inspect2.withFormat("{{.SSHConfig.Port}}")).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession2).To(Exit(0))
		inspectPort2 := inspectSession2.outputToString()
		Expect(inspectPort2).To(Not(Equal(inspectPort)))

		connections2 := new(listSystemConnection)
		connectionsSession2, err := mb.setCmd(connections2.withFormat("{{.URI}}")).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(connectionsSession2).To(Exit(0))
		connectionURLs2 := connectionsSession2.outputToStringSlice()
		connectionPorts2, err := mapToPort(connectionURLs2)
		Expect(err).ToNot(HaveOccurred())
		Expect(connectionPorts2).To(HaveEach(inspectPort2))
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

		s := &startMachine{}
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
			s := &startMachine{}
			startSession1, err = mb.setName(machine1).setCmd(s).setTimeout(time.Minute * 10).run()
			Expect(err).ToNot(HaveOccurred())
		}()
		go func() {
			defer GinkgoRecover()
			defer wg.Done()
			s := &startMachine{}
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
			Expect(startSession2.errorToString()).To(ContainSubstring("%s already starting or running: only one VM can be active at a time", machine1))
		} else {
			Expect(startSession2).To(Exit(0), "first start failed, second start succeed")
			Expect(startSession1.errorToString()).To(ContainSubstring("%s already starting or running: only one VM can be active at a time", machine2))
		}
	})
})

func mapToPort(uris []string) ([]string, error) {
	ports := []string{}

	for _, uri := range uris {
		u, err := url.Parse(uri)
		if err != nil {
			return nil, err
		}

		port := u.Port()
		if port == "" {
			return nil, fmt.Errorf("no port in URI: %s", uri)
		}

		ports = append(ports, port)
	}
	return ports, nil
}
