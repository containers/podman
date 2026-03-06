package e2e_test

import (
	"fmt"

	"github.com/containers/podman/v6/pkg/machine/define"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine os apply", func() {
	It("apply machine", func() {
		if p := testProvider.VMType(); p == define.WSLVirt {
			i := new(initMachine)
			session, err := mb.setCmd(i.withFakeImage(mb)).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(session).To(Exit(0))
		}
		machineName := "foobar"
		a := new(applyMachineOS)
		applySession, err := mb.setName(machineName).setCmd(a.withImage("quay.io/foobar:latest").withRestart()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(applySession.ExitCode()).To(Equal(125))
		switch testProvider.VMType() {
		case define.WSLVirt:
			Expect(applySession.errorToString()).To(ContainSubstring("this command is not supported for WSL"))
		default:
			Expect(applySession.errorToString()).To(ContainSubstring(fmt.Sprintf("%s: VM does not exist", machineName)))
		}
	})
})
