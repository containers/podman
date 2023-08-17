package e2e_test

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine rm", func() {
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
		i := rmMachine{}
		reallyLongName := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		session, err := mb.setName(reallyLongName).setCmd(&i).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
	})

	It("Remove machine", func() {
		i := new(initMachine)
		session, err := mb.setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))
		rm := rmMachine{}
		removeSession, err := mb.setCmd(rm.withForce()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(removeSession).To(Exit(0))

		// Inspecting a non-existent machine should fail
		// which means it is gone
		_, ec, err := mb.toQemuInspectInfo()
		Expect(err).ToNot(HaveOccurred())
		Expect(ec).To(Equal(125))
	})

	It("Remove running machine", func() {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImagePath(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))
		rm := new(rmMachine)

		// Removing a running machine should fail
		stop, err := mb.setCmd(rm).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stop).To(Exit(125))
		Expect(stop.errorToString()).To(ContainSubstring(fmt.Sprintf("vm \"%s\" cannot be destroyed", name)))

		// Removing again with force
		stopAgain, err := mb.setCmd(rm.withForce()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopAgain).To(Exit(0))

		// Inspect to be dead sure
		_, ec, err := mb.toQemuInspectInfo()
		Expect(err).ToNot(HaveOccurred())
		Expect(ec).To(Equal(125))
	})

	It("machine rm --save-keys, --save-ignition, --save-image", func() {

		i := new(initMachine)
		session, err := mb.setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		inspect := new(inspectMachine)
		inspect = inspect.withFormat("{{.SSHConfig.IdentityPath}}")
		inspectSession, err := mb.setCmd(inspect).run()
		Expect(err).ToNot(HaveOccurred())
		key := inspectSession.outputToString()
		pubkey := key + ".pub"

		inspect = inspect.withFormat("{{.Image.IgnitionFile.Path}}")
		inspectSession, err = mb.setCmd(inspect).run()
		Expect(err).ToNot(HaveOccurred())
		ign := inspectSession.outputToString()

		inspect = inspect.withFormat("{{.Image.ImagePath.Path}}")
		inspectSession, err = mb.setCmd(inspect).run()
		Expect(err).ToNot(HaveOccurred())
		img := inspectSession.outputToString()

		rm := rmMachine{}
		removeSession, err := mb.setCmd(rm.withForce().withSaveIgnition().withSaveImage().withSaveKeys()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(removeSession).To(Exit(0))

		// Inspecting a non-existent machine should fail
		// which means it is gone
		_, ec, err := mb.toQemuInspectInfo()
		Expect(err).ToNot(HaveOccurred())
		Expect(ec).To(Equal(125))

		_, err = os.Stat(key)
		Expect(err).ToNot(HaveOccurred())
		_, err = os.Stat(pubkey)
		Expect(err).ToNot(HaveOccurred())
		_, err = os.Stat(ign)
		Expect(err).ToNot(HaveOccurred())
		_, err = os.Stat(img)
		Expect(err).ToNot(HaveOccurred())

	})
})
