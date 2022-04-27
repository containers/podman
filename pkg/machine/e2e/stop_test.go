package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine stop", func() {
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

	It("stop bad name", func() {
		i := stopMachine{}
		reallyLongName := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		session, err := mb.setName(reallyLongName).setCmd(&i).run()
		Expect(err).To(BeNil())
		Expect(session).To(Exit(125))
	})

	It("Stop running machine", func() {
		i := new(initMachine)
		session, err := mb.setCmd(i.withImagePath(mb.imagePath).withNow()).run()
		Expect(err).To(BeNil())
		Expect(session).To(Exit(0))

		stop := new(stopMachine)
		// Removing a running machine should fail
		stopSession, err := mb.setCmd(stop).run()
		Expect(err).To(BeNil())
		Expect(stopSession).To(Exit(0))

		// Stopping it again should not result in an error
		stopAgain, err := mb.setCmd(stop).run()
		Expect(err).To(BeNil())
		Expect(stopAgain).To(Exit((0)))
	})
})
