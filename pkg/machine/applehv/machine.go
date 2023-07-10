//go:build arm64 && darwin

package applehv

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
)

var (
	// vmtype refers to qemu (vs libvirt, krun, etc).
	vmtype = machine.AppleHvVirt
)

// VfkitHelper describes the use of vfkit: cmdline and endpoint
type VfkitHelper struct {
	Bootloader        string
	Devices           []string
	LogLevel          logrus.Level
	PathToVfkitBinary string
	Endpoint          string
}

type MacMachine struct {
	// ConfigPath is the fully qualified path to the configuration file
	ConfigPath machine.VMFile
	// HostUser contains info about host user
	machine.HostUser
	// ImageConfig describes the bootable image
	machine.ImageConfig
	// Mounts is the list of remote filesystems to mount
	Mounts []machine.Mount
	// Name of VM
	Name string
	// TODO We will need something like this for applehv but until host networking
	// is worked out, we cannot be sure what it looks like.
	/*
		// NetworkVSock is for the user networking
		NetworkHVSock machine.HVSockRegistryEntry
		// ReadySocket tells host when vm is booted
		ReadyHVSock HVSockRegistryEntry
		// ResourceConfig is physical attrs of the VM
	*/
	machine.ResourceConfig
	// SSHConfig for accessing the remote vm
	machine.SSHConfig
	// Starting tells us whether the machine is running or if we have just dialed it to start it
	Starting bool
	// Created contains the original created time instead of querying the file mod time
	Created time.Time
	// LastUp contains the last recorded uptime
	LastUp time.Time
	// The VFKit endpoint where we can interact with the VM
	VfkitHelper
}

func (m *MacMachine) Init(opts machine.InitOptions) (bool, error) {
	var (
		key string
	)
	dataDir, err := machine.GetDataDir(machine.AppleHvVirt)
	if err != nil {
		return false, err
	}
	// Acquire the image
	switch opts.ImagePath {
	case machine.Testing.String(), machine.Next.String(), machine.Stable.String(), "":
		g, err := machine.NewGenericDownloader(machine.HyperVVirt, opts.Name, opts.ImagePath)
		if err != nil {
			return false, err
		}

		imagePath, err := machine.NewMachineFile(g.Get().GetLocalUncompressedFile(dataDir), nil)
		if err != nil {
			return false, err
		}
		m.ImagePath = *imagePath
	default:
		// The user has provided an alternate image which can be a file path
		// or URL.
		m.ImageStream = "custom"
		g, err := machine.NewGenericDownloader(vmtype, m.Name, opts.ImagePath)
		if err != nil {
			return false, err
		}
		imagePath, err := machine.NewMachineFile(g.Get().LocalUncompressedFile, nil)
		if err != nil {
			return false, err
		}
		m.ImagePath = *imagePath
		if err := machine.DownloadImage(g); err != nil {
			return false, err
		}
	}

	// Store VFKit stuffs
	vfhelper, err := newVfkitHelper(m.Name, defaultVFKitEndpoint, m.ImagePath.GetPath())
	if err != nil {
		return false, err
	}
	m.VfkitHelper = *vfhelper

	m.IdentityPath = util.GetIdentityPath(m.Name)
	m.Rootful = opts.Rootful
	m.RemoteUsername = opts.Username

	m.UID = os.Getuid()

	// TODO A final decision on networking implementation will need to be made
	// prior to this working
	//sshPort, err := utils.GetRandomPort()
	//if err != nil {
	//	return false, err
	//}
	m.Port = 22

	if len(opts.IgnitionPath) < 1 {
		// TODO localhost needs to be restored here
		uri := machine.SSHRemoteConnection.MakeSSHURL("192.168.64.2", fmt.Sprintf("/run/user/%d/podman/podman.sock", m.UID), strconv.Itoa(m.Port), m.RemoteUsername)
		uriRoot := machine.SSHRemoteConnection.MakeSSHURL("localhost", "/run/podman/podman.sock", strconv.Itoa(m.Port), "root")
		identity := m.IdentityPath

		uris := []url.URL{uri, uriRoot}
		names := []string{m.Name, m.Name + "-root"}

		// The first connection defined when connections is empty will become the default
		// regardless of IsDefault, so order according to rootful
		if opts.Rootful {
			uris[0], names[0], uris[1], names[1] = uris[1], names[1], uris[0], names[0]
		}

		for i := 0; i < 2; i++ {
			if err := machine.AddConnection(&uris[i], names[i], identity, opts.IsDefault && i == 0); err != nil {
				return false, err
			}
		}
	} else {
		fmt.Println("An ignition path was provided.  No SSH connection was added to Podman")
	}

	// TODO resize disk

	if err := m.writeConfig(); err != nil {
		return false, err
	}

	if len(opts.IgnitionPath) < 1 {
		var err error
		key, err = machine.CreateSSHKeys(m.IdentityPath)
		if err != nil {
			return false, err
		}
	}

	if len(opts.IgnitionPath) > 0 {
		inputIgnition, err := os.ReadFile(opts.IgnitionPath)
		if err != nil {
			return false, err
		}
		return false, os.WriteFile(m.IgnitionFile.GetPath(), inputIgnition, 0644)
	}
	// TODO Ignition stuff goes here
	// Write the ignition file
	ign := machine.DynamicIgnition{
		Name:      opts.Username,
		Key:       key,
		VMName:    m.Name,
		VMType:    machine.AppleHvVirt,
		TimeZone:  opts.TimeZone,
		WritePath: m.IgnitionFile.GetPath(),
		UID:       m.UID,
		Rootful:   m.Rootful,
	}

	if err := ign.GenerateIgnitionConfig(); err != nil {
		return false, err
	}
	if err := ign.Write(); err != nil {
		return false, err
	}

	return true, nil
}

func (m *MacMachine) Inspect() (*machine.InspectInfo, error) {
	vmState, err := m.state()
	if err != nil {
		return nil, err
	}
	ii := machine.InspectInfo{
		ConfigPath: m.ConfigPath,
		ConnectionInfo: machine.ConnectionConfig{
			PodmanSocket: nil,
			PodmanPipe:   nil,
		},
		Created: m.Created,
		Image: machine.ImageConfig{
			IgnitionFile: m.IgnitionFile,
			ImageStream:  m.ImageStream,
			ImagePath:    m.ImagePath,
		},
		LastUp: m.LastUp,
		Name:   m.Name,
		Resources: machine.ResourceConfig{
			CPUs:     m.CPUs,
			DiskSize: m.DiskSize,
			Memory:   m.Memory,
		},
		SSHConfig: m.SSHConfig,
		State:     vmState,
	}
	return &ii, nil
}

func (m *MacMachine) Remove(name string, opts machine.RemoveOptions) (string, func() error, error) {
	var (
		files []string
	)

	vmState, err := m.state()
	if err != nil {
		return "", nil, err
	}
	if vmState == machine.Running {
		if !opts.Force {
			return "", nil, fmt.Errorf("invalid state: %s is running", m.Name)
		}
		if err := m.stop(true, true); err != nil {
			return "", nil, err
		}
	}

	if !opts.SaveKeys {
		files = append(files, m.IdentityPath, m.IdentityPath+".pub")
	}
	if !opts.SaveIgnition {
		files = append(files, m.IgnitionFile.GetPath())
	}

	if !opts.SaveImage {
		files = append(files, m.ImagePath.GetPath())
	}

	files = append(files, m.ConfigPath.GetPath())

	confirmationMessage := "\nThe following files will be deleted:\n\n"
	for _, msg := range files {
		confirmationMessage += msg + "\n"
	}

	confirmationMessage += "\n"
	return confirmationMessage, func() error {
		for _, f := range files {
			if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
				logrus.Error(err)
			}
		}
		if err := machine.RemoveConnections(m.Name); err != nil {
			logrus.Error(err)
		}
		if err := machine.RemoveConnections(m.Name + "-root"); err != nil {
			logrus.Error(err)
		}

		// TODO We will need something like this for applehv too i think
		/*
			// Remove the HVSOCK for networking
			if err := m.NetworkHVSock.Remove(); err != nil {
				logrus.Errorf("unable to remove registry entry for %s: %q", m.NetworkHVSock.KeyName, err)
			}

			// Remove the HVSOCK for events
			if err := m.ReadyHVSock.Remove(); err != nil {
				logrus.Errorf("unable to remove registry entry for %s: %q", m.NetworkHVSock.KeyName, err)
			}
		*/
		return nil
	}, nil
}

func (m *MacMachine) writeConfig() error {
	b, err := json.MarshalIndent(m, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.ConfigPath.Path, b, 0644)
}

func (m *MacMachine) Set(name string, opts machine.SetOptions) ([]error, error) {
	var setErrors []error
	vmState, err := m.State(false)
	if err != nil {
		return nil, err
	}
	if vmState != machine.Stopped {
		return nil, machine.ErrWrongState
	}
	if cpus := opts.CPUs; cpus != nil {
		m.CPUs = *cpus
	}
	if mem := opts.Memory; mem != nil {
		m.Memory = *mem
	}
	if newSize := opts.DiskSize; newSize != nil {
		if *newSize < m.DiskSize {
			setErrors = append(setErrors, errors.New("new disk size smaller than existing disk size: cannot shrink disk size"))
		} else {
			m.DiskSize = *newSize
		}
	}

	// Write the machine config to the filesystem
	err = m.writeConfig()
	setErrors = append(setErrors, err)
	switch len(setErrors) {
	case 0:
		return setErrors, nil
	case 1:
		return nil, setErrors[0]
	default:
		// Number of errors is 2 or more
		lastErr := setErrors[len(setErrors)-1]
		return setErrors[:len(setErrors)-1], lastErr
	}
}

func (m *MacMachine) SSH(name string, opts machine.SSHOptions) error {
	st, err := m.State(false)
	if err != nil {
		return err
	}
	if st != machine.Running {
		return fmt.Errorf("vm %q is not running", m.Name)
	}
	username := opts.Username
	if username == "" {
		username = m.RemoteUsername
	}
	// TODO when host networking is figured out, we need to switch this back to
	// machine.commonssh
	return AppleHVSSH(username, m.IdentityPath, m.Name, m.Port, opts.Args)
}

func (m *MacMachine) Start(name string, opts machine.StartOptions) error {
	st, err := m.State(false)
	if err != nil {
		return err
	}
	if st == machine.Running {
		return machine.ErrVMAlreadyRunning
	}
	// TODO Once we decide how to do networking, we can enable the following lines
	// for API forwarding, etc
	//_, _, err = m.startHostNetworking()
	//if err != nil {
	//	return err
	//}
	// To start the VM, we need to call vfkit
	// TODO need to hold the start command until fcos tells us it is started
	return m.VfkitHelper.startVfkit(m)
}

func (m *MacMachine) State(_ bool) (machine.Status, error) {
	vmStatus, err := m.VfkitHelper.state()
	if err != nil {
		return "", err
	}
	return vmStatus, nil
}

func (m *MacMachine) Stop(name string, opts machine.StopOptions) error {
	vmState, err := m.State(false)
	if err != nil {
		return err
	}
	if vmState != machine.Running {
		return machine.ErrWrongState
	}
	return m.VfkitHelper.stop(false, true)
}

// getVMConfigPath is a simple wrapper for getting the fully-qualified
// path of the vm json config file.  It should be used to get conformity
func getVMConfigPath(configDir, vmName string) string {
	return filepath.Join(configDir, fmt.Sprintf("%s.json", vmName))
}
func (m *MacMachine) loadFromFile() (*MacMachine, error) {
	if len(m.Name) < 1 {
		return nil, errors.New("encountered machine with no name")
	}

	jsonPath, err := m.jsonConfigPath()
	if err != nil {
		return nil, err
	}
	mm := MacMachine{}

	if err := loadMacMachineFromJSON(jsonPath, &mm); err != nil {
		return nil, err
	}
	return &mm, nil

}

func loadMacMachineFromJSON(fqConfigPath string, macMachine *MacMachine) error {
	b, err := os.ReadFile(fqConfigPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%q: %w", fqConfigPath, machine.ErrNoSuchVM)
		}
		return err
	}
	return json.Unmarshal(b, macMachine)
}

func (m *MacMachine) jsonConfigPath() (string, error) {
	configDir, err := machine.GetConfDir(machine.AppleHvVirt)
	if err != nil {
		return "", err
	}
	return getVMConfigPath(configDir, m.Name), nil
}

func getVMInfos() ([]*machine.ListResponse, error) {
	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return nil, err
	}

	var listed []*machine.ListResponse

	if err = filepath.WalkDir(vmConfigDir, func(path string, d fs.DirEntry, err error) error {
		vm := new(MacMachine)
		if strings.HasSuffix(d.Name(), ".json") {
			fullPath := filepath.Join(vmConfigDir, d.Name())
			b, err := os.ReadFile(fullPath)
			if err != nil {
				return err
			}
			err = json.Unmarshal(b, vm)
			if err != nil {
				return err
			}
			listEntry := new(machine.ListResponse)

			listEntry.Name = vm.Name
			listEntry.Stream = vm.ImageStream
			listEntry.VMType = machine.AppleHvVirt.String()
			listEntry.CPUs = vm.CPUs
			listEntry.Memory = vm.Memory * units.MiB
			listEntry.DiskSize = vm.DiskSize * units.GiB
			listEntry.Port = vm.Port
			listEntry.RemoteUsername = vm.RemoteUsername
			listEntry.IdentityPath = vm.IdentityPath
			listEntry.CreatedAt = vm.Created
			listEntry.Starting = vm.Starting

			if listEntry.CreatedAt.IsZero() {
				listEntry.CreatedAt = time.Now()
				vm.Created = time.Now()
				if err := vm.writeConfig(); err != nil {
					return err
				}
			}

			vmState, err := vm.State(false)
			if err != nil {
				return err
			}
			listEntry.Running = vmState == machine.Running

			if !vm.LastUp.IsZero() { // this means we have already written a time to the config
				listEntry.LastUp = vm.LastUp
			} else { // else we just created the machine AKA last up = created time
				listEntry.LastUp = vm.Created
				vm.LastUp = listEntry.LastUp
				if err := vm.writeConfig(); err != nil {
					return err
				}
			}

			listed = append(listed, listEntry)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return listed, err
}

func (m *MacMachine) startHostNetworking() (string, machine.APIForwardingState, error) {
	var (
		forwardSock string
		state       machine.APIForwardingState
	)
	cfg, err := config.Default()
	if err != nil {
		return "", machine.NoForwarding, err
	}

	attr := new(os.ProcAttr)
	dnr, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0755)
	if err != nil {
		return "", machine.NoForwarding, err
	}
	dnw, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0755)
	if err != nil {
		return "", machine.NoForwarding, err
	}

	defer func() {
		if err := dnr.Close(); err != nil {
			logrus.Error(err)
		}
	}()
	defer func() {
		if err := dnw.Close(); err != nil {
			logrus.Error(err)
		}
	}()

	gvproxy, err := cfg.FindHelperBinary("gvproxy", false)
	if err != nil {
		return "", 0, err
	}

	attr.Files = []*os.File{dnr, dnw, dnw}
	cmd := []string{gvproxy}
	// Add the ssh port
	cmd = append(cmd, []string{"-ssh-port", fmt.Sprintf("%d", m.Port)}...)
	// TODO Fix when host networking is setup
	//cmd = append(cmd, []string{"-listen", fmt.Sprintf("vsock://%s", m.NetworkHVSock.KeyName)}...)

	cmd, forwardSock, state = m.setupAPIForwarding(cmd)
	if logrus.GetLevel() == logrus.DebugLevel {
		cmd = append(cmd, "--debug")
		fmt.Println(cmd)
	}
	_, err = os.StartProcess(cmd[0], cmd, attr)
	if err != nil {
		return "", 0, fmt.Errorf("unable to execute: %q: %w", cmd, err)
	}
	return forwardSock, state, nil
}

// AppleHVSSH is a temporary function for applehv until we decide how the networking will work
// for certain.
func AppleHVSSH(username, identityPath, name string, sshPort int, inputArgs []string) error {
	sshDestination := username + "@192.168.64.2"
	port := strconv.Itoa(sshPort)

	args := []string{"-i", identityPath, "-p", port, sshDestination,
		"-o", "IdentitiesOnly=yes",
		"-o", "StrictHostKeyChecking=no", "-o", "LogLevel=ERROR", "-o", "SetEnv=LC_ALL="}
	if len(inputArgs) > 0 {
		args = append(args, inputArgs...)
	} else {
		fmt.Printf("Connecting to vm %s. To close connection, use `~.` or `exit`\n", name)
	}

	cmd := exec.Command("ssh", args...)
	logrus.Debugf("Executing: ssh %v\n", args)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}
func (m *MacMachine) setupAPIForwarding(cmd []string) ([]string, string, machine.APIForwardingState) {
	socket, err := m.forwardSocketPath()
	if err != nil {
		return cmd, "", machine.NoForwarding
	}

	destSock := fmt.Sprintf("/run/user/%d/podman/podman.sock", m.UID)
	forwardUser := "core"

	if m.Rootful {
		destSock = "/run/podman/podman.sock"
		forwardUser = "root"
	}

	cmd = append(cmd, []string{"-forward-sock", socket.GetPath()}...)
	cmd = append(cmd, []string{"-forward-dest", destSock}...)
	cmd = append(cmd, []string{"-forward-user", forwardUser}...)
	cmd = append(cmd, []string{"-forward-identity", m.IdentityPath}...)

	return cmd, "", machine.MachineLocal
}

func (m *MacMachine) dockerSock() (string, error) {
	dd, err := machine.GetDataDir(machine.AppleHvVirt)
	if err != nil {
		return "", err
	}
	return filepath.Join(dd, "podman.sock"), nil
}

func (m *MacMachine) forwardSocketPath() (*machine.VMFile, error) {
	sockName := "podman.sock"
	path, err := machine.GetDataDir(machine.AppleHvVirt)
	if err != nil {
		return nil, fmt.Errorf("Resolving data dir: %s", err.Error())
	}
	return machine.NewMachineFile(filepath.Join(path, sockName), &sockName)
}
