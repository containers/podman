package e2e_test

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/Microsoft/go-winio/vhd"
	"github.com/containers/libhvee/pkg/hypervctl"
	"github.com/containers/podman/v6/pkg/machine/define"
	"github.com/containers/podman/v6/pkg/machine/hyperv/vsock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine init - windows only", func() {

	It("init with user mode networking", func() {
		if testProvider.VMType() != define.WSLVirt {
			Skip("test is only supported by WSL")
		}
		i := new(initMachine)
		name := randomString()
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withUserModeNetworking(true)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		defer func() {
			_, err := runWslCommand([]string{"--terminate", "podman-net-usermode"})
			if err != nil {
				fmt.Println("unable to terminate podman-net-usermode")
			}

			_, err = runWslCommand([]string{"--unregister", "podman-net-usermode"})
			if err != nil {
				fmt.Println("unable to unregister podman-net-usermode")
			}
		}()

		inspect := new(inspectMachine)
		inspect = inspect.withFormat("{{.UserModeNetworking}}")
		inspectSession, err := mb.setName(name).setCmd(inspect).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.outputToString()).To(Equal("true"))

		// Ensure port 2222 is free
		listener, err := net.Listen("tcp", "0.0.0.0:2222")
		Expect(err).ToNot(HaveOccurred())
		defer listener.Close()
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
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
		Expect(session.errorToString()).To(ContainSubstring("already exists on hypervisor"))
	})

	It("init should not overwrite existing WSL vms", func() {
		skipIfNotVmtype(define.WSLVirt, "WSL test only")

		name := randomString()
		distName := fmt.Sprintf("podman-%s", name)
		exportedPath := filepath.Join(testDir, "bogus.tar")
		distrDir := filepath.Join(testDir, "testDistro")
		err := os.Mkdir(distrDir, 0o755)
		Expect(err).ToNot(HaveOccurred())

		// create a bogus machine
		i := new(initMachine)
		session, err := mb.setName("foobarexport").setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// export the bogus machine so we have input for making
		// a vm outside the context of podman-machine and also
		// so we dont have to download a distribution from microsoft
		// servers
		exportSession, err := runWslCommand([]string{"--export", "podman-foobarexport", exportedPath})
		Expect(err).ToNot(HaveOccurred())
		Expect(exportSession).To(Exit(0))

		// importing the machine and creating a vm
		importSession, err := runWslCommand([]string{"--import", distName, distrDir, exportedPath})
		Expect(err).ToNot(HaveOccurred())
		Expect(importSession).To(Exit(0))

		defer func() {
			_, err := runWslCommand([]string{"--unregister", distName})
			if err != nil {
				fmt.Println("unable to remove bogus wsl instance")
			}
		}()

		// Trying to make a vm with the same name as an existing name should result in a 125
		checkSession, err := mb.setName(name).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(checkSession).To(Exit(125))
	})

	It("init should create hvsock entries if they do not exist, otherwise should reuse existing ones", func() {
		skipIfNotVmtype(define.HyperVVirt, "HyperV test only")

		name := randomString()

		// Ensure no HVSock entries exist before we start
		networkHvSocks, err := vsock.LoadAllHVSockRegistryEntriesByPurpose(vsock.Network)
		Expect(err).ToNot(HaveOccurred())
		Expect(networkHvSocks).To(BeEmpty())

		readySocks, err := vsock.LoadAllHVSockRegistryEntriesByPurpose(vsock.Events)
		Expect(err).ToNot(HaveOccurred())
		Expect(readySocks).To(BeEmpty())

		// Execute init for the first machine. This should create new HVSock entries
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// Check that the HVSock entries were created
		networkHvSocks, err = vsock.LoadAllHVSockRegistryEntriesByPurpose(vsock.Network)
		Expect(err).ToNot(HaveOccurred())
		Expect(networkHvSocks).To(HaveLen(1))

		readySocks, err = vsock.LoadAllHVSockRegistryEntriesByPurpose(vsock.Events)
		Expect(err).ToNot(HaveOccurred())
		Expect(readySocks).To(HaveLen(1))

		// Execute init	for another machine. This should reuse the existing HVSock entries created above
		otherName := randomString()
		i = new(initMachine)
		session, err = mb.setName(otherName).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		newNetworkHvSocks, err := vsock.LoadAllHVSockRegistryEntriesByPurpose(vsock.Network)
		Expect(err).ToNot(HaveOccurred())
		Expect(newNetworkHvSocks).To(HaveLen(1))
		Expect(newNetworkHvSocks[0].Port).To(Equal(networkHvSocks[0].Port))

		newReadySocks, err := vsock.LoadAllHVSockRegistryEntriesByPurpose(vsock.Events)
		Expect(err).ToNot(HaveOccurred())
		Expect(newReadySocks).To(HaveLen(1))
		Expect(newReadySocks[0].Port).To(Equal(readySocks[0].Port))

		// Clean up all hvsock entries created during the test
		err = vsock.RemoveAllHVSockRegistryEntries()
		Expect(err).ToNot(HaveOccurred())
	})
})
