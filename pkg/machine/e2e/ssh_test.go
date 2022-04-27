package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine ssh", func() {
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

	It("bad machine name", func() {
		name := randomString(12)
		ssh := sshMachine{}
		session, err := mb.setName(name).setCmd(ssh).run()
		Expect(err).To(BeNil())
		Expect(session).To(Exit(125))
		// TODO seems like stderr is not being returned; re-enabled when fixed
		//Expect(session.outputToString()).To(ContainSubstring("not exist"))
	})

	It("ssh to non-running machine", func() {
		name := randomString(12)
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).To(BeNil())
		Expect(session).To(Exit(0))

		ssh := sshMachine{}
		sshSession, err := mb.setName(name).setCmd(ssh).run()
		Expect(err).To(BeNil())
		// TODO seems like stderr is not being returned; re-enabled when fixed
		//Expect(sshSession.outputToString()).To(ContainSubstring("is not running"))
		Expect(sshSession).To(Exit(125))
	})

	It("ssh to running machine and check os-type", func() {
		name := randomString(12)
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImagePath(mb.imagePath).withNow()).run()
		Expect(err).To(BeNil())
		Expect(session).To(Exit(0))

		ssh := sshMachine{}
		sshSession, err := mb.setName(name).setCmd(ssh.withSSHComand([]string{"cat", "/etc/os-release"})).run()
		Expect(err).To(BeNil())
		Expect(sshSession).To(Exit(0))
		Expect(sshSession.outputToString()).To(ContainSubstring("Fedora CoreOS"))
	})
})
