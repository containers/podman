package entities

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/sysinfo"
	"github.com/containers/libpod/pkg/apparmor"
	"github.com/containers/libpod/pkg/cgroups"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/opentracing/opentracing-go"
	"github.com/spf13/pflag"
)

// EngineMode is the connection type podman is using to access libpod
type EngineMode string

const (
	ABIMode    = EngineMode("abi")
	TunnelMode = EngineMode("tunnel")
)

// Convert EngineMode to String
func (m EngineMode) String() string {
	return string(m)
}

// PodmanConfig combines the defaults and settings from the file system with the
// flags given in os.Args. Some runtime state is also stored here.
type PodmanConfig struct {
	*config.Config
	*pflag.FlagSet

	CGroupUsage string           // rootless code determines Usage message
	ConmonPath  string           // --conmon flag will set Engine.ConmonPath
	CpuProfile  string           // Hidden: Should CPU profile be taken
	EngineMode  EngineMode       // ABI or Tunneling mode
	Identities  []string         // ssh identities for connecting to server
	MaxWorks    int              // maximum number of parallel threads
	RuntimePath string           // --runtime flag will set Engine.RuntimePath
	SpanCloser  io.Closer        // Close() for tracing object
	SpanCtx     context.Context  // context to use when tracing
	Span        opentracing.Span // tracing object
	Syslog      bool             // write to StdOut and Syslog, not supported when tunneling
	Trace       bool             // Hidden: Trace execution
	Uri         string           // URI to API Service

	Runroot       string
	StorageDriver string
	StorageOpts   []string
}

// DefaultSecurityOptions: getter for security options from configuration
func (c PodmanConfig) DefaultSecurityOptions() []string {
	securityOpts := []string{}
	if c.Containers.SeccompProfile != "" && c.Containers.SeccompProfile != parse.SeccompDefaultPath {
		securityOpts = append(securityOpts, fmt.Sprintf("seccomp=%s", c.Containers.SeccompProfile))
	}
	if apparmor.IsEnabled() && c.Containers.ApparmorProfile != "" {
		securityOpts = append(securityOpts, fmt.Sprintf("apparmor=%s", c.Containers.ApparmorProfile))
	}
	if selinux.GetEnabled() && !c.Containers.EnableLabeling {
		securityOpts = append(securityOpts, fmt.Sprintf("label=%s", selinux.DisableSecOpt()[0]))
	}
	return securityOpts
}

// DefaultSysctls
func (c PodmanConfig) DefaultSysctls() []string {
	return c.Containers.DefaultSysctls
}

func (c PodmanConfig) DefaultVolumes() []string {
	return c.Containers.Volumes
}

func (c PodmanConfig) DefaultDevices() []string {
	return c.Containers.Devices
}

func (c PodmanConfig) DefaultDNSServers() []string {
	return c.Containers.DNSServers
}

func (c PodmanConfig) DefaultDNSSearches() []string {
	return c.Containers.DNSSearches
}

func (c PodmanConfig) DefaultDNSOptions() []string {
	return c.Containers.DNSOptions
}

func (c PodmanConfig) DefaultEnv() []string {
	return c.Containers.Env
}

func (c PodmanConfig) DefaultInitPath() string {
	return c.Containers.InitPath
}

func (c PodmanConfig) DefaultIPCNS() string {
	return c.Containers.IPCNS
}

func (c PodmanConfig) DefaultPidNS() string {
	return c.Containers.PidNS
}

func (c PodmanConfig) DefaultNetNS() string {
	if c.Containers.NetNS == "private" && rootless.IsRootless() {
		return "slirp4netns"
	}
	return c.Containers.NetNS
}

func (c PodmanConfig) DefaultCgroupNS() string {
	return c.Containers.CgroupNS
}

func (c PodmanConfig) DefaultUTSNS() string {
	return c.Containers.UTSNS
}

func (c PodmanConfig) DefaultShmSize() string {
	return c.Containers.ShmSize
}

func (c PodmanConfig) DefaultUlimits() []string {
	return c.Containers.DefaultUlimits
}

func (c PodmanConfig) DefaultUserNS() string {
	if v, found := os.LookupEnv("PODMAN_USERNS"); found {
		return v
	}
	return c.Containers.UserNS
}

func (c PodmanConfig) DefaultPidsLimit() int64 {
	if rootless.IsRootless() {
		cgroup2, _ := cgroups.IsCgroup2UnifiedMode()
		if cgroup2 {
			return c.Containers.PidsLimit
		}
	}
	return sysinfo.GetDefaultPidsLimit()
}

func (c PodmanConfig) DefaultPidsDescription() string {
	return "Tune container pids limit (set 0 for unlimited)"
}

func (c PodmanConfig) DefaultDetachKeys() string {
	return c.Engine.DetachKeys
}

// TODO: Remove in rootless support PR
// // EngineOptions holds the environment for running the engines
// type EngineOptions struct {
// 	// Introduced with V2
// 	Uri         string
// 	Identities  []string
// 	FlagSet     *pflag.FlagSet
// 	EngineMode  EngineMode
// 	CGroupUsage string
//
// 	// Introduced with V1
// 	CGroupManager     string   // config.EngineConfig
// 	CniConfigDir      string   // config.NetworkConfig.NetworkConfigDir
// 	ConmonPath        string   // config.EngineConfig
// 	DefaultMountsFile string   // config.ContainersConfig
// 	EventsBackend     string   // config.EngineConfig.EventsLogger
// 	HooksDir          []string // config.EngineConfig
// 	MaxWorks          int
// 	Namespace         string // config.EngineConfig
// 	Root              string //
// 	Runroot           string // config.EngineConfig.StorageConfigRunRootSet??
// 	Runtime           string // config.EngineConfig.OCIRuntime
// 	StorageDriver     string // config.EngineConfig.StorageConfigGraphDriverNameSet??
// 	StorageOpts       []string
// 	Syslog            bool
// 	Trace             bool
// 	NetworkCmdPath    string // config.EngineConfig
//
// 	Config     string
// 	CpuProfile string
// 	LogLevel   string
// 	TmpDir     string // config.EngineConfig
//
// 	RemoteUserName       string // deprecated
// 	RemoteHost           string // deprecated
// 	VarlinkAddress       string // deprecated
// 	ConnectionName       string
// 	RemoteConfigFilePath string
// 	Port                 int    // deprecated
// 	IdentityFile         string // deprecated
// 	IgnoreHosts          bool
// }
//
// func NewEngineOptions(opts EngineOptions) (EngineOptions, error) {
// 	ctnrCfg, err := config.Default()
// 	if err != nil {
// 		logrus.Error(err)
// 		os.Exit(1)
// 	}
//
// 	cgroupManager := ctnrCfg.Engine.CgroupManager
// 	cgroupUsage := `Cgroup manager to use ("cgroupfs"|"systemd")`
// 	cgroupv2, _ := cgroups.IsCgroup2UnifiedMode()
// 	cniPluginDir := ctnrCfg.Network.CNIPluginDirs[0]
//
// 	cfg, err := config.NewConfig("")
// 	if err != nil {
// 		logrus.Errorf("Error loading container config %v\n", err)
// 		os.Exit(1)
// 	}
// 	cfg.CheckCgroupsAndAdjustConfig()
//
// 	if rootless.IsRootless() {
// 		if !cgroupv2 {
// 			cgroupManager = ""
// 			cgroupUsage = "Cgroup manager is not supported in rootless mode"
// 		}
// 		cniPluginDir = ""
// 	}
//
// 	return EngineOptions{
// 		CGroupManager:        cgroupManager,
// 		CGroupUsage:          cgroupUsage,
// 		CniConfigDir:         cniPluginDir,
// 		Config:               opts.Config, // TODO: deprecate
// 		ConmonPath:           opts.ConmonPath,
// 		ConnectionName:       opts.ConnectionName,
// 		CpuProfile:           opts.CpuProfile,
// 		DefaultMountsFile:    ctnrCfg.Containers.DefaultMountsFile,
// 		EngineMode:           opts.EngineMode,
// 		EventsBackend:        ctnrCfg.Engine.EventsLogger,
// 		FlagSet:              opts.FlagSet, // TODO: deprecate
// 		HooksDir:             append(ctnrCfg.Engine.HooksDir[:0:0], ctnrCfg.Engine.HooksDir...),
// 		Identities:           append(opts.Identities[:0:0], opts.Identities...),
// 		IdentityFile:         opts.IdentityFile, // TODO: deprecate
// 		IgnoreHosts:          opts.IgnoreHosts,
// 		LogLevel:             opts.LogLevel,
// 		MaxWorks:             opts.MaxWorks,
// 		Namespace:            ctnrCfg.Engine.Namespace,
// 		NetworkCmdPath:       ctnrCfg.Engine.NetworkCmdPath,
// 		Port:                 opts.Port,
// 		RemoteConfigFilePath: opts.RemoteConfigFilePath,
// 		RemoteHost:           opts.RemoteHost,     // TODO: deprecate
// 		RemoteUserName:       opts.RemoteUserName, // TODO: deprecate
// 		Root:                 opts.Root,
// 		Runroot:              opts.Runroot,
// 		Runtime:              opts.Runtime,
// 		StorageDriver:        opts.StorageDriver,
// 		StorageOpts:          append(opts.StorageOpts[:0:0], opts.StorageOpts...),
// 		Syslog:               opts.Syslog,
// 		TmpDir:               opts.TmpDir,
// 		Trace:                opts.Trace,
// 		Uri:                  opts.Uri,
// 		VarlinkAddress:       opts.VarlinkAddress,
// 	}, nil
// }
