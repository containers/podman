package entities

import (
	"os/user"
	"path/filepath"

	"github.com/containers/common/pkg/config"
	"github.com/spf13/pflag"
)

type EngineMode string

const (
	ABIMode    = EngineMode("abi")
	TunnelMode = EngineMode("tunnel")
)

func (m EngineMode) String() string {
	return string(m)
}

type EngineOptions struct {
	Uri        string
	Identities []string
	FlagSet    *pflag.FlagSet
	EngineMode EngineMode

	CGroupManager     string
	CniConfigDir      string
	ConmonPath        string
	DefaultMountsFile string
	EventsBackend     string
	HooksDir          []string
	MaxWorks          int
	Namespace         string
	Root              string
	Runroot           string
	Runtime           string
	StorageDriver     string
	StorageOpts       []string
	Syslog            bool
	Trace             bool
	NetworkCmdPath    string

	Config     string
	CpuProfile string
	LogLevel   string
	TmpDir     string

	RemoteUserName       string
	RemoteHost           string
	VarlinkAddress       string
	ConnectionName       string
	RemoteConfigFilePath string
	Port                 int
	IdentityFile         string
	IgnoreHosts          bool
}

func NewEngineOptions() (EngineOptions, error) {
	u, _ := user.Current()
	return EngineOptions{
		CGroupManager:        config.SystemdCgroupsManager,
		CniConfigDir:         "",
		Config:               "",
		ConmonPath:           filepath.Join("usr", "bin", "conmon"),
		ConnectionName:       "",
		CpuProfile:           "",
		DefaultMountsFile:    "",
		EventsBackend:        "",
		HooksDir:             nil,
		IdentityFile:         "",
		IgnoreHosts:          false,
		LogLevel:             "",
		MaxWorks:             0,
		Namespace:            "",
		NetworkCmdPath:       "",
		Port:                 0,
		RemoteConfigFilePath: "",
		RemoteHost:           "",
		RemoteUserName:       "",
		Root:                 "",
		Runroot:              filepath.Join("run", "user", u.Uid),
		Runtime:              "",
		StorageDriver:        "overlayfs",
		StorageOpts:          nil,
		Syslog:               false,
		TmpDir:               filepath.Join("run", "user", u.Uid, "libpod", "tmp"),
		Trace:                false,
		VarlinkAddress:       "",
	}, nil
}
