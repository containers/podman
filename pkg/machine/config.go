// +build amd64,!windows arm64,!windows

package machine

import (
	"net"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/containers/storage/pkg/homedir"
	"github.com/pkg/errors"
)

type InitOptions struct {
	CPUS         uint64
	DiskSize     uint64
	IgnitionPath string
	ImagePath    string
	IsDefault    bool
	Memory       uint64
	Name         string
	URI          url.URL
	Username     string
}

type RemoteConnectionType string

var (
	SSHRemoteConnection     RemoteConnectionType = "ssh"
	DefaultIgnitionUserName                      = "core"
	ErrNoSuchVM                                  = errors.New("VM does not exist")
	ErrVMAlreadyExists                           = errors.New("VM already exists")
	ErrVMAlreadyRunning                          = errors.New("VM already running")
	ErrMultipleActiveVM                          = errors.New("only one VM can be active at a time")
	ForwarderBinaryName                          = "gvproxy"
)

type Download struct {
	Arch                  string
	Artifact              string
	CompressionType       string
	Format                string
	ImageName             string `json:"image_name"`
	LocalPath             string
	LocalUncompressedFile string
	Sha256sum             string
	URL                   *url.URL
	VMName                string
}

type ListOptions struct{}

type ListResponse struct {
	Name      string
	CreatedAt time.Time
	LastUp    time.Time
	Running   bool
	Stream    string
	VMType    string
	CPUs      uint64
	Memory    uint64
	DiskSize  uint64
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

type VM interface {
	Init(opts InitOptions) error
	Remove(name string, opts RemoveOptions) (string, func() error, error)
	SSH(name string, opts SSHOptions) error
	Start(name string, opts StartOptions) error
	Stop(name string, opts StopOptions) error
}

type DistributionDownload interface {
	DownloadImage() error
	Get() *Download
}

func (rc RemoteConnectionType) MakeSSHURL(host, path, port, userName string) url.URL {
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

// GetDataDir returns the filepath where vm images should
// live for podman-machine
func GetDataDir(vmType string) (string, error) {
	data, err := homedir.GetDataHome()
	if err != nil {
		return "", err
	}
	dataDir := filepath.Join(data, "containers", "podman", "machine", vmType)
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		return dataDir, nil
	}
	mkdirErr := os.MkdirAll(dataDir, 0755)
	return dataDir, mkdirErr
}

// GetConfigDir returns the filepath to where configuration
// files for podman-machine should live
func GetConfDir(vmType string) (string, error) {
	conf, err := homedir.GetConfigHome()
	if err != nil {
		return "", err
	}
	confDir := filepath.Join(conf, "containers", "podman", "machine", vmType)
	if _, err := os.Stat(confDir); !os.IsNotExist(err) {
		return confDir, nil
	}
	mkdirErr := os.MkdirAll(confDir, 0755)
	return confDir, mkdirErr
}
