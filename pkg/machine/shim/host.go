package shim

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/connection"
	machineDefine "github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/env"
	"github.com/containers/podman/v5/pkg/machine/ignition"
	"github.com/containers/podman/v5/pkg/machine/lock"
	"github.com/containers/podman/v5/pkg/machine/proxyenv"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"github.com/containers/podman/v5/utils"
	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
)

// List is done at the host level to allow for a *possible* future where
// more than one provider is used
func List(vmstubbers []vmconfigs.VMProvider, _ machine.ListOptions) ([]*machine.ListResponse, error) {
	var (
		lrs []*machine.ListResponse
	)

	for _, s := range vmstubbers {
		dirs, err := env.GetMachineDirs(s.VMType())
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
				UserModeNetworking: s.UserModeNetworkEnabled(mc),
			}
			lrs = append(lrs, &lr)
		}
	}

	return lrs, nil
}

func Init(opts machineDefine.InitOptions, mp vmconfigs.VMProvider) error {
	var (
		err            error
		imageExtension string
		imagePath      *machineDefine.VMFile
	)

	callbackFuncs := machine.CleanUp()
	defer callbackFuncs.CleanIfErr(&err)
	go callbackFuncs.CleanOnSignal()

	dirs, err := env.GetMachineDirs(mp.VMType())
	if err != nil {
		return err
	}

	sshIdentityPath, err := env.GetSSHIdentityPath(machineDefine.DefaultIdentityName)
	if err != nil {
		return err
	}
	sshKey, err := machine.GetSSHKeys(sshIdentityPath)
	if err != nil {
		return err
	}

	machineLock, err := lock.GetMachineLock(opts.Name, dirs.ConfigDir.GetPath())
	if err != nil {
		return err
	}
	machineLock.Lock()
	defer machineLock.Unlock()

	mc, err := vmconfigs.NewMachineConfig(opts, dirs, sshIdentityPath, mp.VMType(), machineLock)
	if err != nil {
		return err
	}

	mc.Version = vmconfigs.MachineConfigVersion

	createOpts := machineDefine.CreateVMOpts{
		Name: opts.Name,
		Dirs: dirs,
	}

	if umn := opts.UserModeNetworking; umn != nil {
		createOpts.UserModeNetworking = *umn
	}

	// Get Image
	// TODO This needs rework bigtime; my preference is most of below of not living in here.
	// ideally we could get a func back that pulls the image, and only do so IF everything works because
	// image stuff is the slowest part of the operation

	// This is a break from before.  New images are named vmname-ARCH.
	// It turns out that Windows/HyperV will not accept a disk that
	// is not suffixed as ".vhdx". Go figure
	switch mp.VMType() {
	case machineDefine.QemuVirt:
		imageExtension = ".qcow2"
	case machineDefine.AppleHvVirt:
		imageExtension = ".raw"
	case machineDefine.HyperVVirt:
		imageExtension = ".vhdx"
	default:
		// do nothing
	}

	imagePath, err = dirs.DataDir.AppendToNewVMFile(fmt.Sprintf("%s-%s%s", opts.Name, runtime.GOARCH, imageExtension), nil)
	if err != nil {
		return err
	}
	mc.ImagePath = imagePath

	// TODO The following stanzas should be re-written in a differeent place.  It should have a custom
	// parser for our image pulling.  It would be nice if init just got an error and mydisk back.
	//
	// Eventual valid input:
	// "" <- means take the default
	// "http|https://path"
	// "/path
	// "docker://quay.io/something/someManifest

	if err := mp.GetDisk(opts.Image, dirs, mc); err != nil {
		return err
	}

	callbackFuncs.Add(mc.ImagePath.Delete)

	logrus.Debugf("--> imagePath is %q", imagePath.GetPath())

	ignitionFile, err := mc.IgnitionFile()
	if err != nil {
		return err
	}

	uid := os.Getuid()
	if uid == -1 { // windows compensation
		uid = 1000
	}

	// TODO the definition of "user" should go into
	// common for WSL
	userName := opts.Username
	if mp.VMType() == machineDefine.WSLVirt {
		if opts.Username == "core" {
			userName = "user"
			mc.SSH.RemoteUsername = "user"
		}
	}

	ignBuilder := ignition.NewIgnitionBuilder(ignition.DynamicIgnition{
		Name:      userName,
		Key:       sshKey,
		TimeZone:  opts.TimeZone,
		UID:       uid,
		VMName:    opts.Name,
		VMType:    mp.VMType(),
		WritePath: ignitionFile.GetPath(),
		Rootful:   opts.Rootful,
	})

	// If the user provides an ignition file, we need to
	// copy it into the conf dir
	if len(opts.IgnitionPath) > 0 {
		err = ignBuilder.BuildWithIgnitionFile(opts.IgnitionPath)
		return err
	}

	err = ignBuilder.GenerateIgnitionConfig()
	if err != nil {
		return err
	}

	readyIgnOpts, err := mp.PrepareIgnition(mc, &ignBuilder)
	if err != nil {
		return err
	}

	readyUnitFile, err := ignition.CreateReadyUnitFile(mp.VMType(), readyIgnOpts)
	if err != nil {
		return err
	}

	readyUnit := ignition.Unit{
		Enabled:  ignition.BoolToPtr(true),
		Name:     "ready.service",
		Contents: ignition.StrToPtr(readyUnitFile),
	}
	ignBuilder.WithUnit(readyUnit)

	// Mounts
	if mp.VMType() != machineDefine.WSLVirt {
		mc.Mounts = CmdLineVolumesToMounts(opts.Volumes, mp.MountType())
	}

	// TODO AddSSHConnectionToPodmanSocket could take an machineconfig instead
	if err := connection.AddSSHConnectionsToPodmanSocket(mc.HostUser.UID, mc.SSH.Port, mc.SSH.IdentityPath, mc.Name, mc.SSH.RemoteUsername, opts); err != nil {
		return err
	}

	cleanup := func() error {
		return connection.RemoveConnections(mc.Name, mc.Name+"-root")
	}
	callbackFuncs.Add(cleanup)

	err = mp.CreateVM(createOpts, mc, &ignBuilder)
	if err != nil {
		return err
	}

	err = ignBuilder.Build()
	if err != nil {
		return err
	}

	return mc.Write()
}

// VMExists looks across given providers for a machine's existence.  returns the actual config and found bool
func VMExists(name string, vmstubbers []vmconfigs.VMProvider) (*vmconfigs.MachineConfig, bool, error) {
	// Look on disk first
	mcs, err := getMCsOverProviders(vmstubbers)
	if err != nil {
		return nil, false, err
	}
	if mc, found := mcs[name]; found {
		return mc, true, nil
	}
	// Check with the provider hypervisor
	for _, vmstubber := range vmstubbers {
		exists, err := vmstubber.Exists(name)
		if err != nil {
			return nil, false, err
		}
		if exists {
			return nil, true, fmt.Errorf("vm %q already exists on hypervisor", name)
		}
	}
	return nil, false, nil
}

// CheckExclusiveActiveVM checks if any of the machines are already running
func CheckExclusiveActiveVM(provider vmconfigs.VMProvider, mc *vmconfigs.MachineConfig) error {
	// Don't check if provider supports parallel running machines
	if !provider.RequireExclusiveActive() {
		return nil
	}

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
		dirs, err := env.GetMachineDirs(stubber.VMType())
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
func Stop(mc *vmconfigs.MachineConfig, mp vmconfigs.VMProvider, dirs *machineDefine.MachineDirs, hardStop bool) error {
	// state is checked here instead of earlier because stopping a stopped vm is not considered
	// an error.  so putting in one place instead of sprinkling all over.
	mc.Lock()
	defer mc.Unlock()

	return stopLocked(mc, mp, dirs, hardStop)
}

// stopLocked stops the machine and expects the caller to hold the machine's lock.
func stopLocked(mc *vmconfigs.MachineConfig, mp vmconfigs.VMProvider, dirs *machineDefine.MachineDirs, hardStop bool) error {
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
	if !mp.UseProviderNetworkSetup() {
		gvproxyPidFile, err := dirs.RuntimeDir.AppendToNewVMFile("gvproxy.pid", nil)
		if err != nil {
			return err
		}
		if err := machine.CleanupGVProxy(*gvproxyPidFile); err != nil {
			return fmt.Errorf("unable to clean up gvproxy: %w", err)
		}
	}

	// Update last time up
	mc.LastUp = time.Now()
	return mc.Write()
}

func Start(mc *vmconfigs.MachineConfig, mp vmconfigs.VMProvider, dirs *machineDefine.MachineDirs, opts machine.StartOptions) error {
	defaultBackoff := 500 * time.Millisecond
	maxBackoffs := 6

	mc.Lock()
	defer mc.Unlock()

	// Set starting to true
	mc.Starting = true
	if err := mc.Write(); err != nil {
		logrus.Error(err)
	}
	// Set starting to false on exit
	defer func() {
		mc.Starting = false
		if err := mc.Write(); err != nil {
			logrus.Error(err)
		}
	}()

	gvproxyPidFile, err := dirs.RuntimeDir.AppendToNewVMFile("gvproxy.pid", nil)
	if err != nil {
		return err
	}

	// start gvproxy and set up the API socket forwarding
	forwardSocketPath, forwardingState, err := startNetworking(mc, mp)
	if err != nil {
		return err
	}

	callBackFuncs := machine.CleanUp()
	defer callBackFuncs.CleanIfErr(&err)
	go callBackFuncs.CleanOnSignal()

	// Clean up gvproxy if start fails
	cleanGV := func() error {
		return machine.CleanupGVProxy(*gvproxyPidFile)
	}
	callBackFuncs.Add(cleanGV)

	// if there are generic things that need to be done, a preStart function could be added here
	// should it be extensive

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

	if releaseCmd != nil && releaseCmd() != nil { // some providers can return nil here (hyperv)
		if err := releaseCmd(); err != nil {
			// I think it is ok for a "light" error?
			logrus.Error(err)
		}
	}

	if !opts.NoInfo && !mc.HostUser.Rootful {
		machine.PrintRootlessWarning(mc.Name)
	}

	err = mp.PostStartNetworking(mc, opts.NoInfo)
	if err != nil {
		return err
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

	if err := proxyenv.ApplyProxies(mc); err != nil {
		return err
	}

	// mount the volumes to the VM
	if err := mp.MountVolumesToVM(mc, opts.Quiet); err != nil {
		return err
	}

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

	// Provider is responsible for waiting
	if mp.UseProviderNetworkSetup() {
		return nil
	}

	noInfo := opts.NoInfo

	machine.WaitAPIAndPrintInfo(
		forwardingState,
		mc.Name,
		findClaimHelper(),
		forwardSocketPath,
		noInfo,
		mc.HostUser.Rootful,
	)

	return nil
}

func Set(mc *vmconfigs.MachineConfig, mp vmconfigs.VMProvider, opts machineDefine.SetOptions) error {
	mc.Lock()
	defer mc.Unlock()

	if err := mc.Refresh(); err != nil {
		return fmt.Errorf("reload config: %w", err)
	}

	if opts.CPUs != nil {
		mc.Resources.CPUs = *opts.CPUs
	}

	if opts.Memory != nil {
		mc.Resources.Memory = *opts.Memory
	}

	if opts.DiskSize != nil {
		if *opts.DiskSize <= mc.Resources.DiskSize {
			return fmt.Errorf("new disk size must be larger than %d GB", mc.Resources.DiskSize)
		}
		mc.Resources.DiskSize = *opts.DiskSize
	}

	if err := mp.SetProviderAttrs(mc, opts); err != nil {
		return err
	}

	// Update the configuration file last if everything earlier worked
	return mc.Write()
}

func Remove(mc *vmconfigs.MachineConfig, mp vmconfigs.VMProvider, dirs *machineDefine.MachineDirs, opts machine.RemoveOptions) error {
	mc.Lock()
	defer mc.Unlock()

	state, err := mp.State(mc, false)
	if err != nil {
		return err
	}

	if state == machineDefine.Running {
		if !opts.Force {
			return &machineDefine.ErrVMRunningCannotDestroyed{Name: mc.Name}
		}
	}

	rmFiles, genericRm, err := mc.Remove(opts.SaveIgnition, opts.SaveImage)
	if err != nil {
		return err
	}

	providerFiles, providerRm, err := mp.Remove(mc)
	if err != nil {
		return err
	}

	// Add provider specific files to the list
	rmFiles = append(rmFiles, providerFiles...)

	// Important!
	// Nothing can be removed at this point.  The user can still opt out below
	//

	if !opts.Force {
		// Warn user
		confirmationMessage(rmFiles)
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Are you sure you want to continue? [y/N] ")
		answer, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.ToLower(answer)[0] != 'y' {
			return nil
		}
	}

	if state == machineDefine.Running {
		if err := stopLocked(mc, mp, dirs, true); err != nil {
			return err
		}
	}

	//
	// All actual removal of files and vms should occur after this
	//

	if err := providerRm(); err != nil {
		logrus.Errorf("failed to remove virtual machine from provider for %q: %v", mc.Name, err)
	}

	if err := genericRm(); err != nil {
		return fmt.Errorf("failed to remove machines files: %v", err)
	}
	return nil
}

func confirmationMessage(files []string) {
	fmt.Printf("The following files will be deleted:\n\n\n")
	for _, msg := range files {
		fmt.Println(msg)
	}
}

func Reset(dirs *machineDefine.MachineDirs, mp vmconfigs.VMProvider, mcs map[string]*vmconfigs.MachineConfig) error {
	var resetErrors *multierror.Error
	for _, mc := range mcs {
		err := Stop(mc, mp, dirs, true)
		if err != nil {
			resetErrors = multierror.Append(resetErrors, err)
		}
		_, genericRm, err := mc.Remove(false, false)
		if err != nil {
			resetErrors = multierror.Append(resetErrors, err)
		}
		_, providerRm, err := mp.Remove(mc)
		if err != nil {
			resetErrors = multierror.Append(resetErrors, err)
		}

		if err := genericRm(); err != nil {
			resetErrors = multierror.Append(resetErrors, err)
		}
		if err := providerRm(); err != nil {
			resetErrors = multierror.Append(resetErrors, err)
		}
	}

	// Delete the various directories
	// Note: we cannot delete the machine run dir blindly like this because
	// other things live there like the podman.socket and so forth.

	// in linux this ~/.local/share/containers/podman/machine
	dataDirErr := utils.GuardedRemoveAll(filepath.Dir(dirs.DataDir.GetPath()))
	// in linux this ~/.config/containers/podman/machine
	confDirErr := utils.GuardedRemoveAll(filepath.Dir(dirs.ConfigDir.GetPath()))
	resetErrors = multierror.Append(resetErrors, confDirErr, dataDirErr)
	return resetErrors.ErrorOrNil()
}
