package e2e_test

import (
	"io/fs"
	"os"

	"github.com/containers/podman/v4/pkg/machine"
	jsoniter "github.com/json-iterator/go"
	. "github.com/onsi/ginkgo"
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

	It("Remove running machine but leave extra disks intact", func() {
		i := new(initMachine)
		session, err := mb.setCmd(i.withImagePath(mb.imagePath).withExtraDiskNum(1).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))
		rm := new(rmMachine)

		// regular inspect should
		inspectJSON := new(inspectMachine)
		inspectSession, err := mb.setCmd(inspectJSON).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))

		var inspectInfo []machine.InspectInfo
		err = jsoniter.Unmarshal(inspectSession.Bytes(), &inspectInfo)
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectInfo[0].Disks[0].GetPath()).ToNot(BeEmpty())

		defer os.Remove(inspectInfo[0].Disks[0].GetPath())

		// Removing again with force
		stopAgain, err := mb.setCmd(rm.withForce().withSaveDisks()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopAgain).To(Exit(0))

		// Inspect to be dead sure
		_, ec, err := mb.toQemuInspectInfo()
		Expect(err).ToNot(HaveOccurred())
		Expect(ec).To(Equal(125))

		// Make sure the extra disk is still there
		var f fs.FileInfo
		f, err = os.Stat(inspectInfo[0].Disks[0].GetPath())
		Expect(err).ToNot(HaveOccurred())
		Expect(f.Name()).ToNot(BeEmpty())
	})
})
