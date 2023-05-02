package e2e_test

import (
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
		_, err = mb.setCmd(rm.withForce()).run()
		Expect(err).ToNot(HaveOccurred())

		// Inspecting a non-existent machine should fail
		// which means it is gone
		_, ec, err := mb.toQemuInspectInfo()
		Expect(err).ToNot(HaveOccurred())
		Expect(ec).To(Equal(125))
	})

	It("Remove running machine", func() {
		i := new(initMachine)
		session, err := mb.setCmd(i.withImagePath(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))
		rm := new(rmMachine)

		// Removing a running machine should fail
		stop, err := mb.setCmd(rm).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stop).To(Exit(125))

		// Removing again with force
		stopAgain, err := mb.setCmd(rm.withForce()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopAgain).To(Exit(0))

		// Inspect to be dead sure
		_, ec, err := mb.toQemuInspectInfo()
		Expect(err).ToNot(HaveOccurred())
		Expect(ec).To(Equal(125))
	})
})
