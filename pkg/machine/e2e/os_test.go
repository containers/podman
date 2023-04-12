package e2e_test

// import (
// 	. "github.com/onsi/ginkgo/v2"
// 	. "github.com/onsi/gomega"
// 	. "github.com/onsi/gomega/gexec"
// )

// var _ = Describe("podman machine os apply", func() {
// 	var (
// 		mb      *machineTestBuilder
// 		testDir string
// 	)

// 	BeforeEach(func() {
// 		testDir, mb = setup()
// 	})
// 	AfterEach(func() {
// 		teardown(originalHomeDir, testDir, mb)
// 	})

// 	It("apply machine", func() {
// 		i := new(initMachine)
// 		foo1, err := mb.setName("foo1").setCmd(i.withImagePath(mb.imagePath)).run()
// 		Expect(err).ToNot(HaveOccurred())
// 		Expect(foo1).To(Exit(0))

// 		apply := new(applyMachineOS)
// 		applySession, err := mb.setName("foo1").setCmd(apply.args([]string{"quay.io/baude/podman_next"})).run()
// 		Expect(err).ToNot(HaveOccurred())
// 		Expect(applySession).To(Exit(0))
// 	})

// 	It("apply machine from containers-storage", func() {
// 		i := new(initMachine)
// 		foo1, err := mb.setName("foo1").setCmd(i.withImagePath(mb.imagePath)).run()
// 		Expect(err).ToNot(HaveOccurred())
// 		Expect(foo1).To(Exit(0))

// 		ssh := sshMachine{}
// 		sshSession, err := mb.setName("foo1").setCmd(ssh.withSSHComand([]string{"podman", "pull", "quay.io/baude/podman_next"})).run()
// 		Expect(err).ToNot(HaveOccurred())
// 		Expect(sshSession).To(Exit(0))

// 		apply := new(applyMachineOS)
// 		applySession, err := mb.setName("foo1").setCmd(apply.args([]string{"quay.io/baude/podman_next"})).run()
// 		Expect(err).ToNot(HaveOccurred())
// 		Expect(applySession).To(Exit(0))
// 		Expect(applySession.outputToString()).To(ContainSubstring("Pulling from: containers-storage"))
// 	})

// 	It("apply machine not exist", func() {
// 		apply := new(applyMachineOS)
// 		applySession, err := mb.setName("foo1").setCmd(apply.args([]string{"quay.io/baude/podman_next", "notamachine"})).run()
// 		Expect(err).ToNot(HaveOccurred())
// 		Expect(applySession).To(Exit(125))
// 		Expect(applySession.errorToString()).To(ContainSubstring("not exist"))
// 	})
// })
