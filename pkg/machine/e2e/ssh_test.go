package e2e_test

import (
	"github.com/containers/podman/v4/pkg/machine"
	. "github.com/onsi/ginkgo/v2"
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
		name := randomString()
		ssh := sshMachine{}
		session, err := mb.setName(name).setCmd(ssh).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
		Expect(session.errorToString()).To(ContainSubstring("not exist"))
	})

	It("ssh to non-running machine", func() {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		ssh := sshMachine{}
		sshSession, err := mb.setName(name).setCmd(ssh).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession.errorToString()).To(ContainSubstring("is not running"))
		Expect(sshSession).To(Exit(125))
	})

	It("ssh to running machine and check os-type", func() {
		wsl := testProvider.VMType() == machine.WSLVirt
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImagePath(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		ssh := sshMachine{}
		sshSession, err := mb.setName(name).setCmd(ssh.withSSHCommand([]string{"cat", "/etc/os-release"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession).To(Exit(0))

		if wsl {
			Expect(sshSession.outputToString()).To(ContainSubstring("Fedora Linux"))
		} else {
			Expect(sshSession.outputToString()).To(ContainSubstring("Fedora CoreOS"))
		}

		// keep exit code
		sshSession, err = mb.setName(name).setCmd(ssh.withSSHCommand([]string{"false"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession).To(Exit(1))
		Expect(sshSession.outputToString()).To(Equal(""))

		// WSL will often emit an error message about the ssh key and keychains
		if !wsl {
			Expect(sshSession.errorToString()).To(Equal(""))
		}
	})
})
