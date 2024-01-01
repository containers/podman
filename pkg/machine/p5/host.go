package p5

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	"github.com/containers/podman/v4/pkg/machine"
	machineDefine "github.com/containers/podman/v4/pkg/machine/define"
	"github.com/containers/podman/v4/pkg/machine/ocipull"
	"github.com/containers/podman/v4/pkg/machine/vmconfigs"
	"github.com/sirupsen/logrus"
)

/*
Host
   ├ Info
   ├ OS Apply
   ├ SSH
   ├ List
   ├ Init
   ├ VMExists
   ├ CheckExclusiveActiveVM *HyperV/WSL need to check their hypervisors as well
*/

func Info()    {}
func OSApply() {}
func SSH()     {}

// List is done at the host level to allow for a *possible* future where
// more than one provider is used
func List(vmstubbers []vmconfigs.VMStubber) error {
	mcs, err := getMCs(vmstubbers)
	if err != nil {
		return err
	}

	fmt.Println("machines")
	for name, mc := range mcs {
		logrus.Debugf("found machine -> %q %q", name, mc.Created)
	}
	fmt.Println("machines end")

	return nil
}

func Init(opts machineDefine.InitOptions, mp vmconfigs.VMStubber) (*vmconfigs.MachineConfig, error) {
	dirs, err := machine.GetMachineDirs(mp.VMType())
	if err != nil {
		return nil, err
	}
	fmt.Println("/// begin init")

	mc, err := vmconfigs.NewMachineConfig(opts, dirs.ConfigDir)
	if err != nil {
		return nil, err
	}
	createOpts := machineDefine.CreateVMOpts{
		Name: opts.Name,
		Dirs: dirs,
	}

	// Get Image
	// TODO This needs rework bigtime; my preference is most of below of not living in here.
	versionedOCIDownload, err := ocipull.NewVersioned(context.Background(), dirs.DataDir, opts.Name, mp.VMType().String())
	if err != nil {
		return nil, err
	}

	if err := versionedOCIDownload.Pull(); err != nil {
		return nil, err
	}
	unpacked, err := versionedOCIDownload.Unpack()
	if err != nil {
		return nil, err
	}
	defer func() {
		logrus.Debugf("cleaning up %q", unpacked.GetPath())
		if err := unpacked.Delete(); err != nil {
			logrus.Errorf("unable to delete local compressed file %q:%v", unpacked.GetPath(), err)
		}
	}()
	imagePath, err := versionedOCIDownload.Decompress(unpacked)
	if err != nil {
		return nil, err
	}

	mc.ImagePath = imagePath

	// TODO needs callback to remove image

	logrus.Debugf("--> imagePath is %q", imagePath.GetPath())
	// TODO development only -- set to qemu provider
	if err := mp.CreateVM(createOpts, mc); err != nil {
		return nil, err
	}

	b, err := json.MarshalIndent(mc, "", " ")
	if err != nil {
		return nil, err
	}
	fmt.Println(string(b))
	fmt.Println("/// end init")
	return mc, nil
}

// VMExists looks across given providers for a machine's existence.  returns the actual config and found bool
func VMExists(name string, vmstubbers []vmconfigs.VMStubber) (*vmconfigs.MachineConfig, bool, error) {
	mcs, err := getMCs(vmstubbers)
	if err != nil {
		return nil, false, err
	}
	mc, found := mcs[name]
	return mc, found, nil
}

func CheckExclusiveActiveVM() {}

func getMCs(vmstubbers []vmconfigs.VMStubber) (map[string]*vmconfigs.MachineConfig, error) {
	mcs := make(map[string]*vmconfigs.MachineConfig)
	for _, stubber := range vmstubbers {
		dirs, err := machine.GetMachineDirs(stubber.VMType())
		if err != nil {
			return nil, err
		}
		stubberMCs, err := vmconfigs.LoadMachinesInDir(dirs.ConfigDir)
		if err != nil {
			return nil, err
		}
		maps.Copy(mcs, stubberMCs)
	}
	return mcs, nil
}
