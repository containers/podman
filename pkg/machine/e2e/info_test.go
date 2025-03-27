package e2e_test

import (
	"strconv"

	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/goccy/go-json"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine info", func() {
	It("machine info", func() {
		info := new(infoMachine)
		infoSession, err := mb.setCmd(info).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(infoSession).Should(Exit(0))

		// Verify go template works and check for number of machines
		info = new(infoMachine)
		infoSession, err = mb.setCmd(info.withFormat("{{.Host.NumberOfMachines}}")).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(infoSession).Should(Exit(0))
		numMachines, err := strconv.Atoi(infoSession.outputToString())
		Expect(err).ToNot(HaveOccurred())

		// Create a machine and check if info has been updated
		i := new(initMachine)
		initSession, err := mb.setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(initSession).To(Exit(0))

		info = new(infoMachine)
		infoSession, err = mb.setCmd(info.withFormat("{{.Host.NumberOfMachines}}")).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(infoSession).Should(Exit(0))
		Expect(infoSession.outputToString()).To(Equal(strconv.Itoa(numMachines + 1)))

		// Check if json is in correct format
		infoSession, err = mb.setCmd(info.withFormat("json")).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(infoSession).Should(Exit(0))

		infoReport := &entities.MachineInfo{}
		err = json.Unmarshal(infoSession.Bytes(), infoReport)
		Expect(err).ToNot(HaveOccurred())
	})
})
