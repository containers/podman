package e2e_test

import (
	"strings"
	"time"

	"github.com/containers/podman/v5/pkg/domain/entities"
	jsoniter "github.com/json-iterator/go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"golang.org/x/exp/slices"
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
		Expect(firstList.outputToStringSlice()).To(HaveLen(1)) // just the header

		i := new(initMachine)
		session, err := mb.setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		secondList, err := mb.setCmd(list).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(secondList).To(Exit(0))
		Expect(secondList.outputToStringSlice()).To(HaveLen(2)) // one machine and the header
	})

	It("list machines with quiet or noheading", func() {
		// Random names for machines to test list
		name1 := randomString()
		name2 := randomString()

		list := new(listMachine)
		firstList, err := mb.setCmd(list.withQuiet()).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(firstList).Should(Exit(0))
		Expect(firstList.outputToStringSlice()).To(BeEmpty()) // No header with quiet

		noheaderSession, err := mb.setCmd(list.withNoHeading()).run() // noheader
		Expect(err).NotTo(HaveOccurred())
		Expect(noheaderSession).Should(Exit(0))
		Expect(noheaderSession.outputToStringSlice()).To(BeEmpty())

		i := new(initMachine)
		session, err := mb.setName(name1).setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		session2, err := mb.setName(name2).setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session2).To(Exit(0))

		secondList, err := mb.setCmd(list.withQuiet()).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(secondList).To(Exit(0))
		Expect(secondList.outputToStringSlice()).To(HaveLen(2)) // two machines, no header

		listNames := secondList.outputToStringSlice()
		stripAsterisk(listNames)
		Expect(slices.Contains(listNames, name1)).To(BeTrue())
		Expect(slices.Contains(listNames, name2)).To(BeTrue())
	})

	It("list machine: check if running while starting", func() {
		skipIfWSL("the below logic does not work on WSL.  #20978")
		i := new(initMachine)
		session, err := mb.setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		l := new(listMachine)
		listSession, err := mb.setCmd(l.withFormat("{{.LastUp}}")).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(listSession).To(Exit(0))
		Expect(listSession.outputToString()).To(Equal("Never"))

		// The logic in this test stanza is seemingly invalid on WSL.
		// issue #20978 reflects this change
		s := new(startMachine)
		startSession, err := mb.setCmd(s).runWithoutWait()
		Expect(err).ToNot(HaveOccurred())
		wait := 3
		retries := (int)(mb.timeout/time.Second) / wait
		for i := 0; i < retries; i++ {
			listSession, err := mb.setCmd(l).run()
			Expect(listSession).To(Exit(0))
			Expect(err).ToNot(HaveOccurred())
			if startSession.ExitCode() == -1 {
				Expect(listSession.outputToString()).NotTo(ContainSubstring("Currently running"))
			} else {
				break
			}
			time.Sleep(time.Duration(wait) * time.Second)
		}
		Expect(startSession).To(Exit(0))
		listSession, err = mb.setCmd(l).run()
		Expect(listSession).To(Exit(0))
		Expect(err).ToNot(HaveOccurred())
		Expect(listSession.outputToString()).To(ContainSubstring("Currently running"))
		Expect(listSession.outputToString()).NotTo(ContainSubstring("Less than a second ago")) // check to make sure time created is accurate
	})

	It("list with --format", func() {
		// Random names for machines to test list
		name1 := randomString()

		i := new(initMachine)
		session, err := mb.setName(name1).setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// go format
		list := new(listMachine)
		listSession, err := mb.setCmd(list.withFormat("{{.Name}}")).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(listSession).To(Exit(0))
		Expect(listSession.outputToStringSlice()).To(HaveLen(1))

		listNames := listSession.outputToStringSlice()
		stripAsterisk(listNames)
		Expect(slices.Contains(listNames, name1)).To(BeTrue())

		// --format json
		list2 := new(listMachine)
		list2 = list2.withFormat("json")
		listSession2, err := mb.setCmd(list2).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(listSession2).To(Exit(0))
		Expect(listSession2.outputToString()).To(BeValidJSON())

		var listResponse []*entities.ListReporter
		err = jsoniter.Unmarshal(listSession2.Bytes(), &listResponse)
		Expect(err).ToNot(HaveOccurred())

		// table format includes the header
		list = new(listMachine)
		listSession3, err3 := mb.setCmd(list.withFormat("table {{.Name}}")).run()
		Expect(err3).NotTo(HaveOccurred())
		Expect(listSession3).To(Exit(0))
		listNames3 := listSession3.outputToStringSlice()
		Expect(listNames3).To(HaveLen(2))
	})
})

func stripAsterisk(sl []string) {
	for idx, val := range sl {
		sl[idx] = strings.TrimRight(val, "*")
	}
}
