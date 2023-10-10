//go:build amd64 || arm64
// +build amd64 arm64

package machine

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/storage/pkg/homedir"
	"github.com/containers/storage/pkg/lockfile"
	"github.com/sirupsen/logrus"
)

type InitOptions struct {
	CPUS               uint64
	DiskSize           uint64
	IgnitionPath       string
	ImagePath          string
	Volumes            []string
	VolumeDriver       string
	IsDefault          bool
	Memory             uint64
	Name               string
	TimeZone           string
	URI                url.URL
	Username           string
	ReExec             bool
	Rootful            bool
	UID                string // uid of the user that called machine
	UserModeNetworking *bool  // nil = use backend/system default, false = disable, true = enable
}

type Status = string

const (
	// Running indicates the qemu vm is running.
	Running Status = "running"
	// Stopped indicates the vm has stopped.
	Stopped Status = "stopped"
	// Starting indicated the vm is in the process of starting
	Starting Status = "starting"
	// Unknown means the state is not known
	Unknown            Status = "unknown"
	DefaultMachineName string = "podman-machine-default"
	apiUpTimeout              = 20 * time.Second
)

type RemoteConnectionType string

var (
	SSHRemoteConnection     RemoteConnectionType = "ssh"
	DefaultIgnitionUserName                      = "core"
	ForwarderBinaryName                          = "gvproxy"
)

type Download struct {
	Arch                  string
	Artifact              Artifact
	CacheDir              string
	CompressionType       ImageCompression
	DataDir               string
	Format                ImageFormat
	ImageName             string
	LocalPath             string
	LocalUncompressedFile string
	Sha256sum             string
	Size                  int64
	URL                   *url.URL
	VMKind                VMType
	VMName                string
}

type ListOptions struct{}

type ListResponse struct {
	Name               string
	CreatedAt          time.Time
	LastUp             time.Time
	Running            bool
	Starting           bool
	Stream             string
	VMType             string
	CPUs               uint64
	Memory             uint64
	DiskSize           uint64
	Port               int
	RemoteUsername     string
	IdentityPath       string
	UserModeNetworking bool
}

type SetOptions struct {
	CPUs               *uint64
	DiskSize           *uint64
	Memory             *uint64
	Rootful            *bool
	UserModeNetworking *bool
}

type SSHOptions struct {
	Username string
	Args     []string
}

type StartOptions struct {
	NoInfo bool
	Quiet  bool
}

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

func GetLock(name string, vmtype VMType) (*lockfile.LockFile, error) {
	// FIXME: there's a painful amount of `GetConfDir` calls scattered
	// across the code base.  This should be done once and stored
	// somewhere instead.
	vmConfigDir, err := GetConfDir(vmtype)
	if err != nil {
		return nil, err
	}

	lockPath := filepath.Join(vmConfigDir, name+".lock")
	lock, err := lockfile.GetLockFile(lockPath)
	if err != nil {
		return nil, fmt.Errorf("creating lockfile for VM: %w", err)
	}

	return lock, nil
}

type DistributionDownload interface {
	HasUsableCache() (bool, error)
	Get() *Download
	CleanCache() error
}
type InspectInfo struct {
	ConfigPath         VMFile
	ConnectionInfo     ConnectionConfig
	Created            time.Time
	Image              ImageConfig
	LastUp             time.Time
	Name               string
	Resources          ResourceConfig
	SSHConfig          SSHConfig
	State              Status
	UserModeNetworking bool
	Rootful            bool
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

// GetCacheDir returns the dir where VM images are downloaded into when pulled
func GetCacheDir(vmType VMType) (string, error) {
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
func GetDataDir(vmType VMType) (string, error) {
	dataDirPrefix, err := DataDirPrefix()
	if err != nil {
		return "", err
	}
	dataDir := filepath.Join(dataDirPrefix, vmType.String())
	if _, err := os.Stat(dataDir); !errors.Is(err, os.ErrNotExist) {
		return dataDir, nil
	}
	mkdirErr := os.MkdirAll(dataDir, 0755)
	return dataDir, mkdirErr
}

// GetGLobalDataDir returns the root of all backends
// for shared machine data.
func GetGlobalDataDir() (string, error) {
	dataDir, err := DataDirPrefix()
	if err != nil {
		return "", err
	}

	return dataDir, os.MkdirAll(dataDir, 0755)
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
func GetConfDir(vmType VMType) (string, error) {
	confDirPrefix, err := ConfDirPrefix()
	if err != nil {
		return "", err
	}
	confDir := filepath.Join(confDirPrefix, vmType.String())
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

// GuardedRemoveAll functions much like os.RemoveAll but
// will not delete certain catastrophic paths.
func GuardedRemoveAll(path string) error {
	if path == "" || path == "/" {
		return fmt.Errorf("refusing to recursively delete `%s`", path)
	}
	return os.RemoveAll(path)
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
	return os.ReadFile(m.GetPath())
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
	// Whether one of these fields has changed and actions should be taken
	Modified bool `json:"HostUserModified"`
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
	// PodmanPipe is the exported podman service named pipe (Windows hosts only)
	PodmanPipe *VMFile `json:"PodmanPipe"`
}

type VMType int64

const (
	QemuVirt VMType = iota
	WSLVirt
	AppleHvVirt
	HyperVVirt
)

func (v VMType) String() string {
	switch v {
	case WSLVirt:
		return "wsl"
	case AppleHvVirt:
		return "applehv"
	case HyperVVirt:
		return "hyperv"
	}
	return "qemu"
}

type APIForwardingState int

const (
	NoForwarding APIForwardingState = iota
	ClaimUnsupported
	NotInstalled
	MachineLocal
	DockerGlobal
)

func ParseVMType(input string, emptyFallback VMType) (VMType, error) {
	switch strings.TrimSpace(strings.ToLower(input)) {
	case "qemu":
		return QemuVirt, nil
	case "wsl":
		return WSLVirt, nil
	case "applehv":
		return AppleHvVirt, nil
	case "hyperv":
		return HyperVVirt, nil
	case "":
		return emptyFallback, nil
	default:
		return QemuVirt, fmt.Errorf("unknown VMType `%s`", input)
	}
}

type VirtProvider interface { //nolint:interfacebloat
	Artifact() Artifact
	CheckExclusiveActiveVM() (bool, string, error)
	Compression() ImageCompression
	Format() ImageFormat
	IsValidVMName(name string) (bool, error)
	List(opts ListOptions) ([]*ListResponse, error)
	LoadVMByName(name string) (VM, error)
	NewMachine(opts InitOptions) (VM, error)
	NewDownload(vmName string) (Download, error)
	RemoveAndCleanMachines() error
	VMType() VMType
}

type Virtualization struct {
	artifact    Artifact
	compression ImageCompression
	format      ImageFormat
	vmKind      VMType
}

func (p *Virtualization) Artifact() Artifact {
	return p.artifact
}

func (p *Virtualization) Compression() ImageCompression {
	return p.compression
}

func (p *Virtualization) Format() ImageFormat {
	return p.format
}

func (p *Virtualization) VMType() VMType {
	return p.vmKind
}

func (p *Virtualization) NewDownload(vmName string) (Download, error) {
	cacheDir, err := GetCacheDir(p.VMType())
	if err != nil {
		return Download{}, err
	}

	dataDir, err := GetDataDir(p.VMType())
	if err != nil {
		return Download{}, err
	}

	return Download{
		Artifact:        p.Artifact(),
		CacheDir:        cacheDir,
		CompressionType: p.Compression(),
		DataDir:         dataDir,
		Format:          p.Format(),
		VMKind:          p.VMType(),
		VMName:          vmName,
	}, nil
}

func NewVirtualization(artifact Artifact, compression ImageCompression, format ImageFormat, vmKind VMType) Virtualization {
	return Virtualization{
		artifact,
		compression,
		format,
		vmKind,
	}
}

func WaitAndPingAPI(sock string) {
	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(context.Context, string, string) (net.Conn, error) {
				con, err := net.DialTimeout("unix", sock, apiUpTimeout)
				if err != nil {
					return nil, err
				}
				if err := con.SetDeadline(time.Now().Add(apiUpTimeout)); err != nil {
					return nil, err
				}
				return con, nil
			},
		},
	}

	resp, err := client.Get("http://host/_ping")
	if err == nil {
		defer resp.Body.Close()
	}
	if err != nil || resp.StatusCode != 200 {
		logrus.Warn("API socket failed ping test")
	}
}

func (dl Download) NewFcosDownloader(imageStream FCOSStream) (DistributionDownload, error) {
	info, err := dl.GetFCOSDownload(imageStream)
	if err != nil {
		return nil, err
	}
	urlSplit := strings.Split(info.Location, "/")
	dl.ImageName = urlSplit[len(urlSplit)-1]
	downloadURL, err := url.Parse(info.Location)
	if err != nil {
		return nil, err
	}

	// Complete the download struct
	dl.Arch = GetFcosArch()
	// This could be eliminated as a struct and be a generated()
	dl.LocalPath = filepath.Join(dl.CacheDir, dl.ImageName)
	dl.Sha256sum = info.Sha256Sum
	dl.URL = downloadURL
	fcd := FcosDownload{
		Download: dl,
	}
	dataDir, err := GetDataDir(dl.VMKind)
	if err != nil {
		return nil, err
	}
	fcd.Download.LocalUncompressedFile = fcd.GetLocalUncompressedFile(dataDir)
	return fcd, nil
}

// AcquireVMImage determines if the image is already in a FCOS stream. If so,
// retrieves the image path of the uncompressed file. Otherwise, the user has
// provided an alternative image, so we set the image path and download the image.
func (dl Download) AcquireVMImage(imagePath string) (*VMFile, FCOSStream, error) {
	var (
		err           error
		imageLocation *VMFile
		fcosStream    FCOSStream
	)
	switch imagePath {
	// TODO these need to be re-typed as FCOSStreams
	case Testing.String(), Next.String(), Stable.String(), "":
		// Get image as usual
		fcosStream, err = FCOSStreamFromString(imagePath)
		if err != nil {
			return nil, 0, err
		}

		dd, err := dl.NewFcosDownloader(fcosStream)
		if err != nil {
			return nil, 0, err
		}

		imageLocation, err = NewMachineFile(dd.Get().LocalUncompressedFile, nil)
		if err != nil {
			return nil, 0, err
		}

		if err := DownloadImage(dd); err != nil {
			return nil, 0, err
		}
	default:
		// The user has provided an alternate image which can be a file path
		// or URL.
		fcosStream = CustomStream
		imgPath, err := dl.AcquireAlternateImage(imagePath)
		if err != nil {
			return nil, 0, err
		}
		imageLocation = imgPath
	}
	return imageLocation, fcosStream, nil
}
