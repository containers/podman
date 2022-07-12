package e2e

import (
	"github.com/containers/podman/v4/cmd/podman/machine"
	jsoniter "github.com/json-iterator/go"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine info", func() {
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

	It("machine info", func() {
		info := new(infoMachine)
		infoSession, err := mb.setCmd(info).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(infoSession).Should(Exit(0))

		// Verify go template works and check for no running machines
		info = new(infoMachine)
		infoSession, err = mb.setCmd(info.withFormat("{{.Host.NumberOfMachines}}")).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(infoSession).Should(Exit(0))
		Expect(infoSession.outputToString()).To(Equal("0"))

		// Create a machine and check if info has been updated
		i := new(initMachine)
		initSession, err := mb.setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).To(BeNil())
		Expect(initSession).To(Exit(0))

		info = new(infoMachine)
		infoSession, err = mb.setCmd(info.withFormat("{{.Host.NumberOfMachines}}")).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(infoSession).Should(Exit(0))
		Expect(infoSession.outputToString()).To(Equal("1"))

		// Check if json is in correct format
		infoSession, err = mb.setCmd(info.withFormat("json")).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(infoSession).Should(Exit(0))

		infoReport := &machine.Info{}
		err = jsoniter.Unmarshal(infoSession.Bytes(), infoReport)
		Expect(err).To(BeNil())
	})
})
