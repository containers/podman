//go:build amd64 || arm64
// +build amd64 arm64

package machine

import (
	"errors"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/containers/storage/pkg/homedir"
	"github.com/sirupsen/logrus"
)

type InitOptions struct {
	CPUS         uint64
	DiskSize     uint64
	IgnitionPath string
	ImagePath    string
	Volumes      []string
	VolumeDriver string
	IsDefault    bool
	Memory       uint64
	Name         string
	TimeZone     string
	URI          url.URL
	Username     string
	ReExec       bool
	Rootful      bool
	// The numerical userid of the user that called machine
	UID string
}

type Status = string

const (
	// Running indicates the qemu vm is running.
	Running Status = "running"
	// Stopped indicates the vm has stopped.
	Stopped Status = "stopped"
	// Starting indicated the vm is in the process of starting
	Starting           Status = "starting"
	DefaultMachineName string = "podman-machine-default"
)

type Provider interface {
	NewMachine(opts InitOptions) (VM, error)
	LoadVMByName(name string) (VM, error)
	List(opts ListOptions) ([]*ListResponse, error)
	IsValidVMName(name string) (bool, error)
	CheckExclusiveActiveVM() (bool, string, error)
	RemoveAndCleanMachines() error
	VMType() string
}

type RemoteConnectionType string

var (
	SSHRemoteConnection     RemoteConnectionType = "ssh"
	DefaultIgnitionUserName                      = "core"
	ErrNoSuchVM                                  = errors.New("VM does not exist")
	ErrVMAlreadyExists                           = errors.New("VM already exists")
	ErrVMAlreadyRunning                          = errors.New("VM already running or starting")
	ErrMultipleActiveVM                          = errors.New("only one VM can be active at a time")
	ForwarderBinaryName                          = "gvproxy"
)

type Download struct {
	Arch                  string
	Artifact              string
	CompressionType       string
	CacheDir              string
	Format                string
	ImageName             string
	LocalPath             string
	LocalUncompressedFile string
	Sha256sum             string
	URL                   *url.URL
	VMName                string
	Size                  int64
}

type ListOptions struct{}

type ListResponse struct {
	Name           string
	CreatedAt      time.Time
	LastUp         time.Time
	Running        bool
	Starting       bool
	Stream         string
	VMType         string
	CPUs           uint64
	Memory         uint64
	DiskSize       uint64
	Port           int
	RemoteUsername string
	IdentityPath   string
}

type SetOptions struct {
	CPUs     *uint64
	DiskSize *uint64
	Memory   *uint64
	Rootful  *bool
}

type SSHOptions struct {
	Username string
	Args     []string
}
type StartOptions struct{}

type StopOptions struct{}

type RemoveOptions struct {
	Force        bool
	SaveKeys     bool
	SaveImage    bool
	SaveIgnition bool
}

type InspectOptions struct{}

type VM interface {
	Init(opts InitOptions) (bool, error)
	Inspect() (*InspectInfo, error)
	Remove(name string, opts RemoveOptions) (string, func() error, error)
	Set(name string, opts SetOptions) ([]error, error)
	SSH(name string, opts SSHOptions) error
	Start(name string, opts StartOptions) error
	State(bypass bool) (Status, error)
	Stop(name string, opts StopOptions) error
}

type DistributionDownload interface {
	HasUsableCache() (bool, error)
	Get() *Download
	CleanCache() error
}
type InspectInfo struct {
	ConfigPath     VMFile
	ConnectionInfo ConnectionConfig
	Created        time.Time
	Image          ImageConfig
	LastUp         time.Time
	Name           string
	Resources      ResourceConfig
	SSHConfig      SSHConfig
	State          Status
}

func (rc RemoteConnectionType) MakeSSHURL(host, path, port, userName string) url.URL {
	// TODO Should this function have input verification?
	userInfo := url.User(userName)
	uri := url.URL{
		Scheme:     "ssh",
		Opaque:     "",
		User:       userInfo,
		Host:       host,
		Path:       path,
		RawPath:    "",
		ForceQuery: false,
		RawQuery:   "",
		Fragment:   "",
	}
	if len(port) > 0 {
		uri.Host = net.JoinHostPort(uri.Hostname(), port)
	}
	return uri
}

// GetCacheDir returns the dir where VM images are downladed into when pulled
func GetCacheDir(vmType string) (string, error) {
	dataDir, err := GetDataDir(vmType)
	if err != nil {
		return "", err
	}
	cacheDir := filepath.Join(dataDir, "cache")
	if _, err := os.Stat(cacheDir); !errors.Is(err, os.ErrNotExist) {
		return cacheDir, nil
	}
	return cacheDir, os.MkdirAll(cacheDir, 0755)
}

// GetDataDir returns the filepath where vm images should
// live for podman-machine.
func GetDataDir(vmType string) (string, error) {
	dataDirPrefix, err := DataDirPrefix()
	if err != nil {
		return "", err
	}
	dataDir := filepath.Join(dataDirPrefix, vmType)
	if _, err := os.Stat(dataDir); !errors.Is(err, os.ErrNotExist) {
		return dataDir, nil
	}
	mkdirErr := os.MkdirAll(dataDir, 0755)
	return dataDir, mkdirErr
}

// DataDirPrefix returns the path prefix for all machine data files
func DataDirPrefix() (string, error) {
	data, err := homedir.GetDataHome()
	if err != nil {
		return "", err
	}
	dataDir := filepath.Join(data, "containers", "podman", "machine")
	return dataDir, nil
}

// GetConfigDir returns the filepath to where configuration
// files for podman-machine should live
func GetConfDir(vmType string) (string, error) {
	confDirPrefix, err := ConfDirPrefix()
	if err != nil {
		return "", err
	}
	confDir := filepath.Join(confDirPrefix, vmType)
	if _, err := os.Stat(confDir); !errors.Is(err, os.ErrNotExist) {
		return confDir, nil
	}
	mkdirErr := os.MkdirAll(confDir, 0755)
	return confDir, mkdirErr
}

// ConfDirPrefix returns the path prefix for all machine config files
func ConfDirPrefix() (string, error) {
	conf, err := homedir.GetConfigHome()
	if err != nil {
		return "", err
	}
	confDir := filepath.Join(conf, "containers", "podman", "machine")
	return confDir, nil
}

// ResourceConfig describes physical attributes of the machine
type ResourceConfig struct {
	// CPUs to be assigned to the VM
	CPUs uint64
	// Disk size in gigabytes assigned to the vm
	DiskSize uint64
	// Memory in megabytes assigned to the vm
	Memory uint64
}

const maxSocketPathLength int = 103

type VMFile struct {
	// Path is the fully qualified path to a file
	Path string
	// Symlink is a shortened version of Path by using
	// a symlink
	Symlink *string `json:"symlink,omitempty"`
}

// GetPath returns the working path for a machinefile.  it returns
// the symlink unless one does not exist
func (m *VMFile) GetPath() string {
	if m.Symlink == nil {
		return m.Path
	}
	return *m.Symlink
}

// Delete removes the machinefile symlink (if it exists) and
// the actual path
func (m *VMFile) Delete() error {
	if m.Symlink != nil {
		if err := os.Remove(*m.Symlink); err != nil && !errors.Is(err, os.ErrNotExist) {
			logrus.Errorf("unable to remove symlink %q", *m.Symlink)
		}
	}
	if err := os.Remove(m.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Read the contents of a given file and return in []bytes
func (m *VMFile) Read() ([]byte, error) {
	return ioutil.ReadFile(m.GetPath())
}

// NewMachineFile is a constructor for VMFile
func NewMachineFile(path string, symlink *string) (*VMFile, error) {
	if len(path) < 1 {
		return nil, errors.New("invalid machine file path")
	}
	if symlink != nil && len(*symlink) < 1 {
		return nil, errors.New("invalid symlink path")
	}
	mf := VMFile{Path: path}
	if symlink != nil && len(path) > maxSocketPathLength {
		if err := mf.makeSymlink(symlink); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	return &mf, nil
}

// makeSymlink for macOS creates a symlink in $HOME/.podman/
// for a machinefile like a socket
func (m *VMFile) makeSymlink(symlink *string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	sl := filepath.Join(homeDir, ".podman", *symlink)
	// make the symlink dir and throw away if it already exists
	if err := os.MkdirAll(filepath.Dir(sl), 0700); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	m.Symlink = &sl
	return os.Symlink(m.Path, sl)
}

type Mount struct {
	ReadOnly bool
	Source   string
	Tag      string
	Target   string
	Type     string
}

// ImageConfig describes the bootable image for the VM
type ImageConfig struct {
	// IgnitionFile is the path to the filesystem where the
	// ignition file was written (if needs one)
	IgnitionFile VMFile `json:"IgnitionFilePath"`
	// ImageStream is the update stream for the image
	ImageStream string
	// ImageFile is the fq path to
	ImagePath VMFile `json:"ImagePath"`
}

// HostUser describes the host user
type HostUser struct {
	// Whether this machine should run in a rootful or rootless manner
	Rootful bool
	// UID is the numerical id of the user that called machine
	UID int
}

// SSHConfig contains remote access information for SSH
type SSHConfig struct {
	// IdentityPath is the fq path to the ssh priv key
	IdentityPath string
	// SSH port for user networking
	Port int
	// RemoteUsername of the vm user
	RemoteUsername string
}

// ConnectionConfig contains connections like sockets, etc.
type ConnectionConfig struct {
	// PodmanSocket is the exported podman service socket
	PodmanSocket *VMFile `json:"PodmanSocket"`
}
