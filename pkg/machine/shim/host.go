package shim

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/connection"
	machineDefine "github.com/containers/podman/v4/pkg/machine/define"
	"github.com/containers/podman/v4/pkg/machine/ignition"
	"github.com/containers/podman/v4/pkg/machine/ocipull"
	"github.com/containers/podman/v4/pkg/machine/stdpull"
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
func List(vmstubbers []vmconfigs.VMProvider, opts machine.ListOptions) ([]*machine.ListResponse, error) {
	var (
		lrs []*machine.ListResponse
	)

	for _, s := range vmstubbers {
		dirs, err := machine.GetMachineDirs(s.VMType())
		if err != nil {
			return nil, err
		}
		mcs, err := vmconfigs.LoadMachinesInDir(dirs)
		if err != nil {
			return nil, err
		}
		for name, mc := range mcs {
			state, err := s.State(mc, false)
			if err != nil {
				return nil, err
			}
			lr := machine.ListResponse{
				Name:      name,
				CreatedAt: mc.Created,
				LastUp:    mc.LastUp,
				Running:   state == machineDefine.Running,
				Starting:  mc.Starting,
				//Stream:             "", // No longer applicable
				VMType:             s.VMType().String(),
				CPUs:               mc.Resources.CPUs,
				Memory:             mc.Resources.Memory,
				DiskSize:           mc.Resources.DiskSize,
				Port:               mc.SSH.Port,
				RemoteUsername:     mc.SSH.RemoteUsername,
				IdentityPath:       mc.SSH.IdentityPath,
				UserModeNetworking: false, // TODO Need to plumb this for WSL
			}
			lrs = append(lrs, &lr)
		}
	}

	return lrs, nil
}

func Init(opts machineDefine.InitOptions, mp vmconfigs.VMProvider) (*vmconfigs.MachineConfig, error) {
	var (
		err error
	)
	callbackFuncs := machine.InitCleanup()
	defer callbackFuncs.CleanIfErr(&err)
	go callbackFuncs.CleanOnSignal()

	dirs, err := machine.GetMachineDirs(mp.VMType())
	if err != nil {
		return nil, err
	}

	sshIdentityPath, err := machine.GetSSHIdentityPath(machineDefine.DefaultIdentityName)
	if err != nil {
		return nil, err
	}
	sshKey, err := machine.GetSSHKeys(sshIdentityPath)
	if err != nil {
		return nil, err
	}

	mc, err := vmconfigs.NewMachineConfig(opts, dirs, sshIdentityPath)
	if err != nil {
		return nil, err
	}

	createOpts := machineDefine.CreateVMOpts{
		Name: opts.Name,
		Dirs: dirs,
	}

	// Get Image
	// TODO This needs rework bigtime; my preference is most of below of not living in here.
	// ideally we could get a func back that pulls the image, and only do so IF everything works because
	// image stuff is the slowest part of the operation

	// This is a break from before.  New images are named vmname-ARCH.
	// TODO does the image name need to retain its type? (qcow2)
	imagePath, err := dirs.DataDir.AppendToNewVMFile(fmt.Sprintf("%s-%s", opts.Name, runtime.GOARCH), nil)
	if err != nil {
		return nil, err
	}

	var mydisk ocipull.Disker

	// TODO The following stanzas should be re-written in a differeent place.  It should have a custom
	// parser for our image pulling.  It would be nice if init just got an error and mydisk back.
	//
	// Eventual valid input:
	// "" <- means take the default
	// "http|https://path"
	// "/path
	// "docker://quay.io/something/someManifest

	if opts.ImagePath == "" {
		mydisk, err = ocipull.NewVersioned(context.Background(), dirs.DataDir, opts.Name, mp.VMType().String(), imagePath)
	} else {
		if strings.HasPrefix(opts.ImagePath, "http") {
			// TODO probably should use tempdir instead of datadir
			mydisk, err = stdpull.NewDiskFromURL(opts.ImagePath, imagePath, dirs.DataDir)
		} else {
			mydisk, err = stdpull.NewStdDiskPull(opts.ImagePath, imagePath)
		}
	}
	if err != nil {
		return nil, err
	}
	if err := mydisk.Get(); err != nil {
		return nil, err
	}

	mc.ImagePath = imagePath
	callbackFuncs.Add(mc.ImagePath.Delete)

	logrus.Debugf("--> imagePath is %q", imagePath.GetPath())

	ignitionFile, err := mc.IgnitionFile()
	if err != nil {
		return nil, err
	}

	ignBuilder := ignition.NewIgnitionBuilder(ignition.DynamicIgnition{
		Name:      opts.Username,
		Key:       sshKey,
		TimeZone:  opts.TimeZone,
		UID:       os.Getuid(),
		VMName:    opts.Name,
		VMType:    mp.VMType(),
		WritePath: ignitionFile.GetPath(),
		Rootful:   opts.Rootful,
	})

	// If the user provides an ignition file, we need to
	// copy it into the conf dir
	if len(opts.IgnitionPath) > 0 {
		err = ignBuilder.BuildWithIgnitionFile(opts.IgnitionPath)
		return nil, err
	}

	if err := ignBuilder.GenerateIgnitionConfig(); err != nil {
		return nil, err
	}

	readyUnitFile, err := ignition.CreateReadyUnitFile(machineDefine.QemuVirt, nil)
	if err != nil {
		return nil, err
	}

	readyUnit := ignition.Unit{
		Enabled:  ignition.BoolToPtr(true),
		Name:     "ready.service",
		Contents: ignition.StrToPtr(readyUnitFile),
	}
	ignBuilder.WithUnit(readyUnit)

	if err := ignBuilder.Build(); err != nil {
		return nil, err
	}

	// Mounts
	mc.Mounts = vmconfigs.CmdLineVolumesToMounts(opts.Volumes, mp.MountType())

	// TODO AddSSHConnectionToPodmanSocket could take an machineconfig instead
	if err := connection.AddSSHConnectionsToPodmanSocket(mc.HostUser.UID, mc.SSH.Port, mc.SSH.IdentityPath, mc.Name, mc.SSH.RemoteUsername, opts); err != nil {
		return nil, err
	}

	cleanup := func() error {
		return connection.RemoveConnections(mc.Name, mc.Name+"-root")
	}
	callbackFuncs.Add(cleanup)

	if err := mp.CreateVM(createOpts, mc); err != nil {
		return nil, err
	}

	return mc, err
}

// VMExists looks across given providers for a machine's existence.  returns the actual config and found bool
func VMExists(name string, vmstubbers []vmconfigs.VMProvider) (*vmconfigs.MachineConfig, bool, error) {
	mcs, err := getMCsOverProviders(vmstubbers)
	if err != nil {
		return nil, false, err
	}
	mc, found := mcs[name]
	return mc, found, nil
}

// CheckExclusiveActiveVM checks if any of the machines are already running
func CheckExclusiveActiveVM(provider vmconfigs.VMProvider, mc *vmconfigs.MachineConfig) error {
	// Check if any other machines are running; if so, we error
	localMachines, err := getMCsOverProviders([]vmconfigs.VMProvider{provider})
	if err != nil {
		return err
	}
	for name, localMachine := range localMachines {
		state, err := provider.State(localMachine, false)
		if err != nil {
			return err
		}
		if state == machineDefine.Running {
			return fmt.Errorf("unable to start %q: machine %s already running", mc.Name, name)
		}
	}
	return nil
}

// getMCsOverProviders loads machineconfigs from a config dir derived from the "provider".  it returns only what is known on
// disk so things like status may be incomplete or inaccurate
func getMCsOverProviders(vmstubbers []vmconfigs.VMProvider) (map[string]*vmconfigs.MachineConfig, error) {
	mcs := make(map[string]*vmconfigs.MachineConfig)
	for _, stubber := range vmstubbers {
		dirs, err := machine.GetMachineDirs(stubber.VMType())
		if err != nil {
			return nil, err
		}
		stubberMCs, err := vmconfigs.LoadMachinesInDir(dirs)
		if err != nil {
			return nil, err
		}
		// TODO When we get to golang-1.20+ we can replace the following with maps.Copy
		// maps.Copy(mcs, stubberMCs)
		// iterate known mcs and add the stubbers
		for mcName, mc := range stubberMCs {
			if _, ok := mcs[mcName]; !ok {
				mcs[mcName] = mc
			}
		}
	}
	return mcs, nil
}

// Stop stops the machine as well as supporting binaries/processes
// TODO: I think this probably needs to go somewhere that remove can call it.
func Stop(mc *vmconfigs.MachineConfig, mp vmconfigs.VMProvider, dirs *machineDefine.MachineDirs, hardStop bool) error {
	// state is checked here instead of earlier because stopping a stopped vm is not considered
	// an error.  so putting in one place instead of sprinkling all over.
	state, err := mp.State(mc, false)
	if err != nil {
		return err
	}
	// stopping a stopped machine is NOT an error
	if state == machineDefine.Stopped {
		return nil
	}
	if state != machineDefine.Running {
		return machineDefine.ErrWrongState
	}

	// Provider stops the machine
	if err := mp.StopVM(mc, hardStop); err != nil {
		return err
	}

	// Remove Ready Socket
	readySocket, err := mc.ReadySocket()
	if err != nil {
		return err
	}
	if err := readySocket.Delete(); err != nil {
		return err
	}

	// Stop GvProxy and remove PID file
	gvproxyPidFile, err := dirs.RuntimeDir.AppendToNewVMFile("gvproxy.pid", nil)
	if err != nil {
		return err
	}

	defer func() {
		if err := machine.CleanupGVProxy(*gvproxyPidFile); err != nil {
			logrus.Errorf("unable to clean up gvproxy: %q", err)
		}
	}()

	return nil
}

func Start(mc *vmconfigs.MachineConfig, mp vmconfigs.VMProvider, dirs *machineDefine.MachineDirs, opts machine.StartOptions) error {
	defaultBackoff := 500 * time.Millisecond
	maxBackoffs := 6

	// start gvproxy and set up the API socket forwarding
	forwardSocketPath, forwardingState, err := startNetworking(mc, mp)
	if err != nil {
		return err
	}

	// if there are generic things that need to be done, a preStart function could be added here
	// should it be extensive

	// update the podman/docker socket service if the host user has been modified at all (UID or Rootful)
	if mc.HostUser.Modified {
		if machine.UpdatePodmanDockerSockService(mc) == nil {
			// Reset modification state if there are no errors, otherwise ignore errors
			// which are already logged
			mc.HostUser.Modified = false
			if err := mc.Write(); err != nil {
				logrus.Error(err)
			}
		}
	}

	// releaseFunc is if the provider starts a vm using a go command
	// and we still need control of it while it is booting until the ready
	// socket is tripped
	releaseCmd, WaitForReady, err := mp.StartVM(mc)
	if err != nil {
		return err
	}

	if WaitForReady == nil {
		return errors.New("no valid wait function returned")
	}

	if err := WaitForReady(); err != nil {
		return err
	}

	if releaseCmd() != nil { // overkill but protective
		if err := releaseCmd(); err != nil {
			// I think it is ok for a "light" error?
			logrus.Error(err)
		}
	}

	stateF := func() (machineDefine.Status, error) {
		return mp.State(mc, true)
	}

	connected, sshError, err := conductVMReadinessCheck(mc, maxBackoffs, defaultBackoff, stateF)
	if err != nil {
		return err
	}

	if !connected {
		msg := "machine did not transition into running state"
		if sshError != nil {
			return fmt.Errorf("%s: ssh error: %v", msg, sshError)
		}
		return errors.New(msg)
	}

	// mount the volumes to the VM
	if err := mp.MountVolumesToVM(mc, opts.Quiet); err != nil {
		return err
	}

	machine.WaitAPIAndPrintInfo(
		forwardingState,
		mc.Name,
		findClaimHelper(),
		forwardSocketPath,
		opts.NoInfo,
		mc.HostUser.Rootful,
	)
	return nil
}
