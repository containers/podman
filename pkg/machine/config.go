package machine

import (
	"net"
	"net/url"
	"os"
	"path/filepath"

	"github.com/containers/storage/pkg/homedir"
)

type CreateOptions struct {
	Name         string
	CPUS         uint64
	Memory       uint64
	IgnitionPath string
	ImagePath    string
	Username     string
	URI          url.URL
	IsDefault    bool
	//KernelPath string
	//Devices    []VMDevices
}

type RemoteConnectionType string

var (
	SSHRemoteConnection     RemoteConnectionType = "ssh"
	DefaultIgnitionUserName                      = "core"
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

type SSHOptions struct{}
type StartOptions struct{}

type StopOptions struct{}

type DestroyOptions struct {
	Force        bool
	SaveKeys     bool
	SaveImage    bool
	SaveIgnition bool
}

type VM interface {
	Create(opts CreateOptions) error
	Destroy(name string, opts DestroyOptions) (string, func() error, error)
	SSH(name string, opts SSHOptions) error
	Start(name string, opts StartOptions) error
	Stop(name string, opts StopOptions) error
}

type DistributionDownload interface {
	DownloadImage() error
	Get() *Download
}

// TODO is this even needed?
type TestVM struct{}

func (vm *TestVM) Create(opts CreateOptions) error {
	return nil
}

func (vm *TestVM) Start(name string, opts StartOptions) error {
	return nil
}
func (vm *TestVM) Stop(name string, opts StopOptions) error {
	return nil
}

func (rc RemoteConnectionType) MakeSSHURL(host, path, port, userName string) url.URL {
	userInfo := url.User(userName)
	uri := url.URL{
		Scheme:      "ssh",
		Opaque:      "",
		User:        userInfo,
		Host:        host,
		Path:        path,
		RawPath:     "",
		ForceQuery:  false,
		RawQuery:    "",
		Fragment:    "",
		RawFragment: "",
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
