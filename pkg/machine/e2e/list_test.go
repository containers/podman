package e2e

import (
	"strings"

	"github.com/containers/buildah/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine list", func() {
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

	It("list machine", func() {
		list := new(listMachine)
		firstList, err := mb.setCmd(list).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(firstList).Should(Exit(0))
		Expect(len(firstList.outputToStringSlice())).To(Equal(1)) // just the header

		i := new(initMachine)
		session, err := mb.setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).To(BeNil())
		Expect(session).To(Exit(0))

		secondList, err := mb.setCmd(list).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(secondList).To(Exit(0))
		Expect(len(secondList.outputToStringSlice())).To(Equal(2)) // one machine and the header
	})

	It("list machines with quiet", func() {
		// Random names for machines to test list
		name1 := randomString(12)
		name2 := randomString(12)

		list := new(listMachine)
		firstList, err := mb.setCmd(list.withQuiet()).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(firstList).Should(Exit(0))
		Expect(len(firstList.outputToStringSlice())).To(Equal(0)) // No header with quiet

		i := new(initMachine)
		session, err := mb.setName(name1).setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).To(BeNil())
		Expect(session).To(Exit(0))

		session2, err := mb.setName(name2).setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).To(BeNil())
		Expect(session2).To(Exit(0))

		secondList, err := mb.setCmd(list.withQuiet()).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(secondList).To(Exit(0))
		Expect(len(secondList.outputToStringSlice())).To(Equal(2)) // two machines, no header

		listNames := secondList.outputToStringSlice()
		stripAsterisk(listNames)
		Expect(util.StringInSlice(name1, listNames)).To(BeTrue())
		Expect(util.StringInSlice(name2, listNames)).To(BeTrue())
	})

	It("list machine: check if running while starting", func() {
		i := new(initMachine)
		session, err := mb.setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).To(BeNil())
		Expect(session).To(Exit(0))
		s := new(startMachine)
		startSession, err := mb.setCmd(s).runWithoutWait()
		Expect(err).To(BeNil())
		l := new(listMachine)
		for { // needs to be infinite because we need to check if running when inspect returns to avoid race conditions.
			listSession, err := mb.setCmd(l).run()
			Expect(listSession).To(Exit(0))
			Expect(err).To(BeNil())
			if startSession.ExitCode() == -1 {
				Expect(listSession.outputToString()).NotTo(ContainSubstring("Currently running"))
			} else {
				break
			}
		}
		Expect(startSession).To(Exit(0))
		listSession, err := mb.setCmd(l).run()
		Expect(listSession).To(Exit(0))
		Expect(err).To(BeNil())
		Expect(listSession.outputToString()).To(ContainSubstring("Currently running"))
		Expect(listSession.outputToString()).NotTo(ContainSubstring("Less than a second ago")) // check to make sure time created is accurate
	})
})

func stripAsterisk(sl []string) {
	for idx, val := range sl {
		sl[idx] = strings.TrimRight(val, "*")
	}
}
