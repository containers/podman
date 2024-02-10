package e2e_test

import (
	"fmt"
	"path/filepath"

	"github.com/Microsoft/go-winio/vhd"
	"github.com/containers/libhvee/pkg/hypervctl"
	"github.com/containers/podman/v5/pkg/machine/define"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine init - windows only", func() {
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

	It("init with user mode networking", func() {
		if testProvider.VMType() != define.WSLVirt {
			Skip("test is only supported by WSL")
		}
		i := new(initMachine)
		name := randomString()
		session, err := mb.setName(name).setCmd(i.withImagePath(mb.imagePath).withUserModeNetworking(true)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		inspect := new(inspectMachine)
		inspect = inspect.withFormat("{{.UserModeNetworking}}")
		inspectSession, err := mb.setName(name).setCmd(inspect).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.outputToString()).To(Equal("true"))
	})
	It("init should not should not overwrite existing HyperV vms", func() {
		skipIfNotVmtype(define.HyperVVirt, "HyperV test only")
		name := randomString()
		vhdxPath := filepath.Join(testDir, fmt.Sprintf("%s.vhdx", name))
		if err := vhd.CreateVhdx(vhdxPath, 1, 1); err != nil {
			Fail(fmt.Sprintf("failed to create dummy vhdx %q: %q", vhdxPath, err))
		}
		vmm := hypervctl.NewVirtualMachineManager()
		hwConfig := hypervctl.HardwareConfig{
			CPUs:     1,
			DiskPath: vhdxPath,
			DiskSize: 0,
			Memory:   0,
			Network:  false,
		}

		if err := vmm.NewVirtualMachine(name, &hwConfig); err != nil {
			Fail(fmt.Sprintf("failed to create vm %q: %q", name, err))
		}
		vm, err := vmm.GetMachine(name)
		if err != nil {
			Fail(fmt.Sprintf("failed to get vm %q: %q", name, err))
		}
		defer func() {
			if err := vm.Remove(""); err != nil {
				fmt.Printf("failed to clean up vm %q from hypervisor: %q /n", name, err)
			}
		}()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(session).To(Exit(125))
		Expect(session.errorToString()).To(ContainSubstring("already exists on hypervisor"))
	})
})
