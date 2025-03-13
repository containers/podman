package e2e_test

import (
	"github.com/containers/podman/v5/pkg/machine/define"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine ssh", func() {

	It("bad machine name", func() {
		name := randomString()
		ssh := &sshMachine{}
		session, err := mb.setName(name).setCmd(ssh).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
		Expect(session.errorToString()).To(ContainSubstring("not exist"))
	})

	It("ssh to non-running machine", func() {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		ssh := &sshMachine{}
		sshSession, err := mb.setName(name).setCmd(ssh).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession.errorToString()).To(ContainSubstring("is not running"))
		Expect(sshSession).To(Exit(125))
	})

	It("ssh to running machine and check os-type", func() {
		wsl := testProvider.VMType() == define.WSLVirt
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		ssh := &sshMachine{}
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

	It("verify machine rootfulness", func() {
		wsl := testProvider.VMType() == define.WSLVirt
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		ssh := &sshMachine{}
		sshSession, err := mb.setName(name).setCmd(ssh.withSSHCommand([]string{"whoami"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession).To(Exit(0))
		if wsl {
			Expect(sshSession.outputToString()).To(Equal("user"))
		} else {
			Expect(sshSession.outputToString()).To(Equal("core"))
		}

		stop := &stopMachine{}
		stopSession, err := mb.setName(name).setCmd(stop).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopSession).To(Exit(0))

		set := &setMachine{}
		setSession, err := mb.setName(name).setCmd(set.withRootful(true)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(setSession).To(Exit(0))

		start := &startMachine{}
		startSession, err := mb.setName(name).setCmd(start).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))

		sshSession, err = mb.setName(name).setCmd(ssh.withSSHCommand([]string{"whoami"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession).To(Exit(0))
		Expect(sshSession.outputToString()).To(Equal("root"))
	})
})
