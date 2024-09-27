//go:build amd64 || arm64

package machine

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/containers/common/pkg/strongunits"
	"github.com/containers/podman/v5/pkg/machine/compression"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/env"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"github.com/sirupsen/logrus"
)

const apiUpTimeout = 20 * time.Second

var (
	ForwarderBinaryName = "gvproxy"
)

type Download struct {
	Arch                  string
	Artifact              define.Artifact
	CacheDir              string
	CompressionType       compression.ImageCompression
	DataDir               string
	Format                define.ImageFormat
	ImageName             string
	LocalPath             string
	LocalUncompressedFile string
	Sha256sum             string
	Size                  int64
	URL                   *url.URL
	VMKind                define.VMType
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
	Memory             strongunits.MiB
	DiskSize           strongunits.GiB
	Port               int
	RemoteUsername     string
	IdentityPath       string
	UserModeNetworking bool
}

type SSHOptions struct {
	Username string
	Args     []string
}

type StartOptions struct {
	NoInfo  bool
	Quiet   bool
	Rosetta bool
}

type StopOptions struct{}

type RemoveOptions struct {
	Force        bool
	SaveImage    bool
	SaveIgnition bool
}

type ResetOptions struct {
	Force bool
}

type InspectOptions struct{}

// TODO This can be removed when WSL is refactored into podman 5
type VM interface {
	Init(opts define.InitOptions) (bool, error)
	Inspect() (*InspectInfo, error)
	Remove(name string, opts RemoveOptions) (string, func() error, error)
	Set(name string, opts define.SetOptions) ([]error, error)
	SSH(name string, opts SSHOptions) error
	Start(name string, opts StartOptions) error
	State(bypass bool) (define.Status, error)
	Stop(name string, opts StopOptions) error
}

type DistributionDownload interface {
	HasUsableCache() (bool, error)
	Get() *Download
	CleanCache() error
}
type InspectInfo struct {
	ConfigDir          define.VMFile
	ConnectionInfo     ConnectionConfig
	Created            time.Time
	LastUp             time.Time
	Name               string
	Resources          vmconfigs.ResourceConfig
	SSHConfig          vmconfigs.SSHConfig
	State              define.Status
	UserModeNetworking bool
	Rootful            bool
	Rosetta            bool
}

// ImageConfig describes the bootable image for the VM
type ImageConfig struct {
	// IgnitionFile is the path to the filesystem where the
	// ignition file was written (if needs one)
	IgnitionFile define.VMFile `json:"IgnitionFilePath"`
	// ImageStream is the update stream for the image
	ImageStream string
	// ImageFile is the fq path to
	ImagePath define.VMFile `json:"ImagePath"`
}

// ConnectionConfig contains connections like sockets, etc.
type ConnectionConfig struct {
	// PodmanSocket is the exported podman service socket
	PodmanSocket *define.VMFile `json:"PodmanSocket"`
	// PodmanPipe is the exported podman service named pipe (Windows hosts only)
	PodmanPipe *define.VMFile `json:"PodmanPipe"`
}

type APIForwardingState int

const (
	NoForwarding APIForwardingState = iota
	ClaimUnsupported
	NotInstalled
	MachineLocal
	DockerGlobal
)

// TODO THis should be able to be removed once WSL is refactored for podman5
type Virtualization struct {
	artifact    define.Artifact
	compression compression.ImageCompression
	format      define.ImageFormat
	vmKind      define.VMType
}

func (p *Virtualization) Artifact() define.Artifact {
	return p.artifact
}

func (p *Virtualization) Compression() compression.ImageCompression {
	return p.compression
}

func (p *Virtualization) Format() define.ImageFormat {
	return p.format
}

func (p *Virtualization) VMType() define.VMType {
	return p.vmKind
}

func (p *Virtualization) NewDownload(vmName string) (Download, error) {
	cacheDir, err := env.GetCacheDir(p.VMType())
	if err != nil {
		return Download{}, err
	}

	dataDir, err := env.GetDataDir(p.VMType())
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

func NewVirtualization(artifact define.Artifact, compression compression.ImageCompression, format define.ImageFormat, vmKind define.VMType) Virtualization {
	return Virtualization{
		artifact,
		compression,
		format,
		vmKind,
	}
}

func dialSocket(socket string, timeout time.Duration) (net.Conn, error) {
	scheme := "unix"
	if strings.Contains(socket, "://") {
		url, err := url.Parse(socket)
		if err != nil {
			return nil, err
		}
		scheme = url.Scheme
		socket = url.Path
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var dial func() (net.Conn, error)
	switch scheme {
	default:
		fallthrough
	case "unix":
		dial = func() (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socket)
		}
	case "npipe":
		dial = func() (net.Conn, error) {
			return DialNamedPipe(ctx, socket)
		}
	}

	backoff := 500 * time.Millisecond
	for {
		conn, err := dial()
		if !errors.Is(err, os.ErrNotExist) {
			return conn, err
		}

		select {
		case <-time.After(backoff):
			backoff *= 2
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func WaitAndPingAPI(sock string) {
	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(context.Context, string, string) (net.Conn, error) {
				con, err := dialSocket(sock, apiUpTimeout)
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
	if err != nil || resp.StatusCode != http.StatusOK {
		logrus.Warn("API socket failed ping test")
	}
}
