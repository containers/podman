package e2e_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("run basic podman commands", func() {
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

	It("Basic ops", func() {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImagePath(mb.imagePath).withNow()).run()
		Expect(err).To(BeNil())
		Expect(session).To(Exit(0))

		bm := basicMachine{}
		imgs, err := mb.setCmd(bm.withPodmanCommand([]string{"images", "-q"})).run()
		Expect(err).To(BeNil())
		Expect(imgs).To(Exit(0))
		Expect(len(imgs.outputToStringSlice())).To(Equal(0))

		newImgs, err := mb.setCmd(bm.withPodmanCommand([]string{"pull", "quay.io/libpod/alpine_nginx"})).run()
		Expect(err).To(BeNil())
		Expect(newImgs).To(Exit(0))
		Expect(len(newImgs.outputToStringSlice())).To(Equal(1))

		runAlp, err := mb.setCmd(bm.withPodmanCommand([]string{"run", "quay.io/libpod/alpine_nginx", "cat", "/etc/os-release"})).run()
		Expect(err).To(BeNil())
		Expect(runAlp).To(Exit(0))
		Expect(runAlp.outputToString()).To(ContainSubstring("Alpine Linux"))

		rmCon, err := mb.setCmd(bm.withPodmanCommand([]string{"rm", "-a"})).run()
		Expect(err).To(BeNil())
		Expect(rmCon).To(Exit(0))
	})

})
