package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	nettypes "github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/apparmor"
	"github.com/containers/common/pkg/cgroupv2"
	"github.com/containers/common/pkg/util"
	"github.com/containers/storage/pkg/homedir"
	"github.com/containers/storage/pkg/unshare"
	"github.com/containers/storage/types"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/sirupsen/logrus"
)

const (
	// _defaultGraphRoot points to the default path of the graph root.
	_defaultGraphRoot = "/var/lib/containers/storage"

	// _defaultTransport is a prefix that we apply to an image name to check
	// docker hub first for the image.
	_defaultTransport = "docker://"

	// _defaultImageVolumeMode is a mode to handle built-in image volumes.
	_defaultImageVolumeMode = "bind"
)

var (
	// DefaultInitPath is the default path to the container-init binary.
	DefaultInitPath = "/usr/libexec/podman/catatonit"
	// DefaultInfraImage is the default image to run as infrastructure containers in pods.
	DefaultInfraImage = ""
	// DefaultRootlessSHMLockPath is the default path for rootless SHM locks.
	DefaultRootlessSHMLockPath = "/libpod_rootless_lock"
	// DefaultDetachKeys is the default keys sequence for detaching a
	// container.
	DefaultDetachKeys = "ctrl-p,ctrl-q"
	// ErrConmonOutdated indicates the version of conmon found (whether via the configuration or $PATH)
	// is out of date for the current podman version.
	ErrConmonOutdated = errors.New("outdated conmon version")
	// ErrInvalidArg indicates that an invalid argument was passed.
	ErrInvalidArg = errors.New("invalid argument")
	// DefaultHooksDirs defines the default hooks directory.
	DefaultHooksDirs = []string{"/usr/share/containers/oci/hooks.d"}
	// DefaultCapabilities is the default for the default_capabilities option in the containers.conf file.
	DefaultCapabilities = []string{
		"CAP_CHOWN",
		"CAP_DAC_OVERRIDE",
		"CAP_FOWNER",
		"CAP_FSETID",
		"CAP_KILL",
		"CAP_NET_BIND_SERVICE",
		"CAP_SETFCAP",
		"CAP_SETGID",
		"CAP_SETPCAP",
		"CAP_SETUID",
		"CAP_SYS_CHROOT",
	}

	// Search these locations in which CNIPlugins can be installed.
	DefaultCNIPluginDirs = []string{
		"/usr/local/libexec/cni",
		"/usr/libexec/cni",
		"/usr/local/lib/cni",
		"/usr/lib/cni",
		"/opt/cni/bin",
	}
	DefaultSubnetPools = []SubnetPool{
		// 10.89.0.0/24-10.255.255.0/24
		parseSubnetPool("10.89.0.0/16", 24),
		parseSubnetPool("10.90.0.0/15", 24),
		parseSubnetPool("10.92.0.0/14", 24),
		parseSubnetPool("10.96.0.0/11", 24),
		parseSubnetPool("10.128.0.0/9", 24),
	}
	// additionalHelperBinariesDir is an extra helper binaries directory that
	// should be set during link-time, if different packagers put their
	// helper binary in a different location.
	additionalHelperBinariesDir string
)

// nolint:unparam
func parseSubnetPool(subnet string, size int) SubnetPool {
	_, n, _ := net.ParseCIDR(subnet)
	return SubnetPool{
		Base: &nettypes.IPNet{IPNet: *n},
		Size: size,
	}
}

const (
	// _etcDir is the sysconfdir where podman should look for system config files.
	// It can be overridden at build time.
	_etcDir = "/etc"
	// InstallPrefix is the prefix where podman will be installed.
	// It can be overridden at build time.
	_installPrefix = "/usr"
	// CgroupfsCgroupsManager represents cgroupfs native cgroup manager.
	CgroupfsCgroupsManager = "cgroupfs"
	// DefaultApparmorProfile  specifies the default apparmor profile for the container.
	DefaultApparmorProfile = apparmor.Profile
	// DefaultHostsFile is the default path to the hosts file.
	DefaultHostsFile = "/etc/hosts"
	// SystemdCgroupsManager represents systemd native cgroup manager.
	SystemdCgroupsManager = "systemd"
	// DefaultLogSizeMax is the default value for the maximum log size
	// allowed for a container. Negative values mean that no limit is imposed.
	DefaultLogSizeMax = -1
	// DefaultEventsLogSize is the default value for the maximum events log size
	// before rotation.
	DefaultEventsLogSizeMax = uint64(1000000)
	// DefaultPidsLimit is the default value for maximum number of processes
	// allowed inside a container.
	DefaultPidsLimit = 2048
	// DefaultPullPolicy pulls the image if it does not exist locally.
	DefaultPullPolicy = "missing"
	// DefaultSubnet is the subnet that will be used for the default
	// network.
	DefaultSubnet = "10.88.0.0/16"
	// DefaultRootlessSignaturePolicyPath is the location within
	// XDG_CONFIG_HOME of the rootless policy.json file.
	DefaultRootlessSignaturePolicyPath = "containers/policy.json"
	// DefaultShmSize is the default upper limit on the size of tmpfs mounts.
	DefaultShmSize = "65536k"
	// DefaultUserNSSize indicates the default number of UIDs allocated for user namespace within a container.
	// Deprecated: no user of this field is known.
	DefaultUserNSSize = 65536
	// OCIBufSize limits maximum LogSizeMax.
	OCIBufSize = 8192
	// SeccompOverridePath if this exists it overrides the default seccomp path.
	SeccompOverridePath = _etcDir + "/containers/seccomp.json"
	// SeccompDefaultPath defines the default seccomp path.
	SeccompDefaultPath = _installPrefix + "/share/containers/seccomp.json"
	// DefaultVolumePluginTimeout is the default volume plugin timeout, in seconds
	DefaultVolumePluginTimeout = 5
)

// DefaultConfig defines the default values from containers.conf.
func DefaultConfig() (*Config, error) {
	defaultEngineConfig, err := defaultConfigFromMemory()
	if err != nil {
		return nil, err
	}

	defaultEngineConfig.SignaturePolicyPath = DefaultSignaturePolicyPath
	if useUserConfigLocations() {
		configHome, err := homedir.GetConfigHome()
		if err != nil {
			return nil, err
		}
		sigPath := filepath.Join(configHome, DefaultRootlessSignaturePolicyPath)
		defaultEngineConfig.SignaturePolicyPath = sigPath
		if _, err := os.Stat(sigPath); err != nil {
			if _, err := os.Stat(DefaultSignaturePolicyPath); err == nil {
				defaultEngineConfig.SignaturePolicyPath = DefaultSignaturePolicyPath
			}
		}
	}

	cgroupNS := "host"
	if cgroup2, _ := cgroupv2.Enabled(); cgroup2 {
		cgroupNS = "private"
	}

	return &Config{
		Containers: ContainersConfig{
			Devices:             []string{},
			Volumes:             []string{},
			Annotations:         []string{},
			ApparmorProfile:     DefaultApparmorProfile,
			BaseHostsFile:       "",
			CgroupNS:            cgroupNS,
			Cgroups:             getDefaultCgroupsMode(),
			DefaultCapabilities: DefaultCapabilities,
			DefaultSysctls:      []string{},
			DefaultUlimits:      getDefaultProcessLimits(),
			DNSServers:          []string{},
			DNSOptions:          []string{},
			DNSSearches:         []string{},
			EnableKeyring:       true,
			EnableLabeling:      selinuxEnabled(),
			Env: []string{
				"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"TERM=xterm",
			},
			EnvHost:    false,
			HTTPProxy:  true,
			Init:       false,
			InitPath:   "",
			IPCNS:      "shareable",
			LogDriver:  defaultLogDriver(),
			LogSizeMax: DefaultLogSizeMax,
			NetNS:      "private",
			NoHosts:    false,
			PidsLimit:  DefaultPidsLimit,
			PidNS:      "private",
			ShmSize:    DefaultShmSize,
			TZ:         "",
			Umask:      "0022",
			UTSNS:      "private",
			UserNSSize: DefaultUserNSSize, // Deprecated
		},
		Network: NetworkConfig{
			DefaultNetwork:     "podman",
			DefaultSubnet:      DefaultSubnet,
			DefaultSubnetPools: DefaultSubnetPools,
			DNSBindPort:        0,
			CNIPluginDirs:      DefaultCNIPluginDirs,
		},
		Engine:  *defaultEngineConfig,
		Secrets: defaultSecretConfig(),
		Machine: defaultMachineConfig(),
	}, nil
}

// defaultSecretConfig returns the default secret configuration.
// Please note that the default is choosing the "file" driver.
func defaultSecretConfig() SecretConfig {
	return SecretConfig{
		Driver: "file",
	}
}

// defaultMachineConfig returns the default machine configuration.
func defaultMachineConfig() MachineConfig {
	return MachineConfig{
		CPUs:     1,
		DiskSize: 100,
		Image:    getDefaultMachineImage(),
		Memory:   2048,
		User:     getDefaultMachineUser(),
		Volumes:  getDefaultMachineVolumes(),
	}
}

// defaultConfigFromMemory returns a default engine configuration. Note that the
// config is different for root and rootless. It also parses the storage.conf.
func defaultConfigFromMemory() (*EngineConfig, error) {
	c := new(EngineConfig)
	tmp, err := defaultTmpDir()
	if err != nil {
		return nil, err
	}
	c.TmpDir = tmp

	c.EventsLogFileMaxSize = eventsLogMaxSize(DefaultEventsLogSizeMax)

	c.CompatAPIEnforceDockerHub = true

	if path, ok := os.LookupEnv("CONTAINERS_STORAGE_CONF"); ok {
		if err := types.SetDefaultConfigFilePath(path); err != nil {
			return nil, err
		}
	}
	storeOpts, err := types.DefaultStoreOptions(useUserConfigLocations(), unshare.GetRootlessUID())
	if err != nil {
		return nil, err
	}

	if storeOpts.GraphRoot == "" {
		logrus.Warnf("Storage configuration is unset - using hardcoded default graph root %q", _defaultGraphRoot)
		storeOpts.GraphRoot = _defaultGraphRoot
	}

	c.graphRoot = storeOpts.GraphRoot
	c.ImageCopyTmpDir = getDefaultTmpDir()
	c.StaticDir = filepath.Join(storeOpts.GraphRoot, "libpod")
	c.VolumePath = filepath.Join(storeOpts.GraphRoot, "volumes")

	c.VolumePluginTimeout = DefaultVolumePluginTimeout

	c.HelperBinariesDir = defaultHelperBinariesDir
	if additionalHelperBinariesDir != "" {
		c.HelperBinariesDir = append(c.HelperBinariesDir, additionalHelperBinariesDir)
	}
	c.HooksDir = DefaultHooksDirs
	c.ImageDefaultTransport = _defaultTransport
	c.ImageVolumeMode = _defaultImageVolumeMode
	c.StateType = BoltDBStateStore

	c.ImageBuildFormat = "oci"

	c.CgroupManager = defaultCgroupManager()
	c.ServiceTimeout = uint(5)
	c.StopTimeout = uint(10)
	c.ExitCommandDelay = uint(5 * 60)
	c.Remote = isRemote()
	c.OCIRuntimes = map[string][]string{
		"crun": {
			"/usr/bin/crun",
			"/usr/sbin/crun",
			"/usr/local/bin/crun",
			"/usr/local/sbin/crun",
			"/sbin/crun",
			"/bin/crun",
			"/run/current-system/sw/bin/crun",
		},
		"crun-wasm": {
			"/usr/bin/crun-wasm",
			"/usr/sbin/crun-wasm",
			"/usr/local/bin/crun-wasm",
			"/usr/local/sbin/crun-wasm",
			"/sbin/crun-wasm",
			"/bin/crun-wasm",
			"/run/current-system/sw/bin/crun-wasm",
		},
		"runc": {
			"/usr/bin/runc",
			"/usr/sbin/runc",
			"/usr/local/bin/runc",
			"/usr/local/sbin/runc",
			"/sbin/runc",
			"/bin/runc",
			"/usr/lib/cri-o-runc/sbin/runc",
			"/run/current-system/sw/bin/runc",
		},
		"runj": {
			"/usr/local/bin/runj",
		},
		"kata": {
			"/usr/bin/kata-runtime",
			"/usr/sbin/kata-runtime",
			"/usr/local/bin/kata-runtime",
			"/usr/local/sbin/kata-runtime",
			"/sbin/kata-runtime",
			"/bin/kata-runtime",
			"/usr/bin/kata-qemu",
			"/usr/bin/kata-fc",
		},
		"runsc": {
			"/usr/bin/runsc",
			"/usr/sbin/runsc",
			"/usr/local/bin/runsc",
			"/usr/local/sbin/runsc",
			"/bin/runsc",
			"/sbin/runsc",
			"/run/current-system/sw/bin/runsc",
		},
		"youki": {
			"/usr/local/bin/youki",
			"/usr/bin/youki",
			"/bin/youki",
			"/run/current-system/sw/bin/youki",
		},
		"krun": {
			"/usr/bin/krun",
			"/usr/local/bin/krun",
		},
		"ocijail": {
			"/usr/local/bin/ocijail",
		},
	}
	c.PlatformToOCIRuntime = map[string]string{
		"wasi/wasm":   "crun-wasm",
		"wasi/wasm32": "crun-wasm",
		"wasi/wasm64": "crun-wasm",
	}
	// Needs to be called after populating c.OCIRuntimes.
	c.OCIRuntime = c.findRuntime()

	c.ConmonEnvVars = []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	}
	c.ConmonPath = []string{
		"/usr/libexec/podman/conmon",
		"/usr/local/libexec/podman/conmon",
		"/usr/local/lib/podman/conmon",
		"/usr/bin/conmon",
		"/usr/sbin/conmon",
		"/usr/local/bin/conmon",
		"/usr/local/sbin/conmon",
		"/run/current-system/sw/bin/conmon",
	}
	c.ConmonRsPath = []string{
		"/usr/libexec/podman/conmonrs",
		"/usr/local/libexec/podman/conmonrs",
		"/usr/local/lib/podman/conmonrs",
		"/usr/bin/conmonrs",
		"/usr/sbin/conmonrs",
		"/usr/local/bin/conmonrs",
		"/usr/local/sbin/conmonrs",
		"/run/current-system/sw/bin/conmonrs",
	}
	c.PullPolicy = DefaultPullPolicy
	c.RuntimeSupportsJSON = []string{
		"crun",
		"runc",
		"kata",
		"runsc",
		"youki",
		"krun",
	}
	c.RuntimeSupportsNoCgroups = []string{"crun", "krun"}
	c.RuntimeSupportsKVM = []string{"kata", "kata-runtime", "kata-qemu", "kata-fc", "krun"}
	c.InitPath = DefaultInitPath
	c.NoPivotRoot = false

	c.InfraImage = DefaultInfraImage
	c.EnablePortReservation = true
	c.NumLocks = 2048
	c.EventsLogger = defaultEventsLogger()
	c.DetachKeys = DefaultDetachKeys
	c.SDNotify = false
	// TODO - ideally we should expose a `type LockType string` along with
	// constants.
	c.LockType = getDefaultLockType()
	c.MachineEnabled = false
	c.ChownCopiedFiles = true

	c.PodExitPolicy = defaultPodExitPolicy
	c.SSHConfig = getDefaultSSHConfig()

	return c, nil
}

func defaultTmpDir() (string, error) {
	if !useUserConfigLocations() {
		return getLibpodTmpDir(), nil
	}

	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return "", err
	}
	libpodRuntimeDir := filepath.Join(runtimeDir, "libpod")

	if err := os.Mkdir(libpodRuntimeDir, 0o700|os.ModeSticky); err != nil {
		if !os.IsExist(err) {
			return "", err
		} else if err := os.Chmod(libpodRuntimeDir, 0o700|os.ModeSticky); err != nil {
			// The directory already exists, so we try to make sure it's private and has the sticky bit set on it.
			return "", fmt.Errorf("set sticky bit on: %w", err)
		}
	}
	return filepath.Join(libpodRuntimeDir, "tmp"), nil
}

// NetNS returns the default network namespace.
func (c *Config) NetNS() string {
	return c.Containers.NetNS
}

func (c EngineConfig) EventsLogMaxSize() uint64 {
	return uint64(c.EventsLogFileMaxSize)
}

// SecurityOptions returns the default security options.
func (c *Config) SecurityOptions() []string {
	securityOpts := []string{}
	if c.Containers.SeccompProfile != "" && c.Containers.SeccompProfile != SeccompDefaultPath {
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

// Sysctls returns the default sysctls to set in containers.
func (c *Config) Sysctls() []string {
	return c.Containers.DefaultSysctls
}

// Volumes returns the default set of volumes that should be mounted in containers.
func (c *Config) Volumes() []string {
	return c.Containers.Volumes
}

// Devices returns the default additional devices for containers.
func (c *Config) Devices() []string {
	return c.Containers.Devices
}

// DNSServers returns the default DNS servers to add to resolv.conf in containers.
func (c *Config) DNSServers() []string {
	return c.Containers.DNSServers
}

// DNSSerches returns the default DNS searches to add to resolv.conf in containers.
func (c *Config) DNSSearches() []string {
	return c.Containers.DNSSearches
}

// DNSOptions returns the default DNS options to add to resolv.conf in containers.
func (c *Config) DNSOptions() []string {
	return c.Containers.DNSOptions
}

// Env returns the default additional environment variables to add to containers.
func (c *Config) Env() []string {
	return c.Containers.Env
}

// InitPath returns location where init program added to containers when users specify the --init flag.
func (c *Config) InitPath() string {
	return c.Containers.InitPath
}

// IPCNS returns the default IPC Namespace configuration to run containers with.
func (c *Config) IPCNS() string {
	return c.Containers.IPCNS
}

// PIDNS returns the default PID Namespace configuration to run containers with.
func (c *Config) PidNS() string {
	return c.Containers.PidNS
}

// CgroupNS returns the default Cgroup Namespace configuration to run containers with.
func (c *Config) CgroupNS() string {
	return c.Containers.CgroupNS
}

// Cgroups returns whether to run containers in their own control groups, as configured by the "cgroups" setting in containers.conf.
func (c *Config) Cgroups() string {
	return c.Containers.Cgroups
}

// UTSNS returns the default UTS Namespace configuration to run containers with.
func (c *Config) UTSNS() string {
	return c.Containers.UTSNS
}

// ShmSize returns the default size for temporary file systems to use in containers.
func (c *Config) ShmSize() string {
	return c.Containers.ShmSize
}

// Ulimits returns the default ulimits to use in containers.
func (c *Config) Ulimits() []string {
	return c.Containers.DefaultUlimits
}

// PidsLimit returns the default maximum number of pids to use in containers.
func (c *Config) PidsLimit() int64 {
	if unshare.IsRootless() {
		if c.Engine.CgroupManager != SystemdCgroupsManager {
			return 0
		}
		cgroup2, _ := cgroupv2.Enabled()
		if !cgroup2 {
			return 0
		}
	}

	return c.Containers.PidsLimit
}

// DetachKeys returns the default detach keys to detach from a container.
func (c *Config) DetachKeys() string {
	return c.Engine.DetachKeys
}

// TZ returns the timezone to set in containers.
func (c *Config) TZ() string {
	return c.Containers.TZ
}

func (c *Config) Umask() string {
	return c.Containers.Umask
}

// LogDriver returns the logging driver to be used
// currently k8s-file or journald.
func (c *Config) LogDriver() string {
	return c.Containers.LogDriver
}

// MachineEnabled returns if podman is running inside a VM or not.
func (c *Config) MachineEnabled() bool {
	return c.Engine.MachineEnabled
}

// MachineVolumes returns volumes to mount into the VM.
func (c *Config) MachineVolumes() ([]string, error) {
	return machineVolumes(c.Machine.Volumes)
}

func machineVolumes(volumes []string) ([]string, error) {
	translatedVolumes := []string{}
	for _, v := range volumes {
		vol := os.ExpandEnv(v)
		split := strings.Split(vol, ":")
		if len(split) < 2 || len(split) > 3 {
			return nil, fmt.Errorf("invalid machine volume %s, 2 or 3 fields required", v)
		}
		if split[0] == "" || split[1] == "" {
			return nil, fmt.Errorf("invalid machine volume %s, fields must container data", v)
		}
		translatedVolumes = append(translatedVolumes, vol)
	}
	return translatedVolumes, nil
}

func getDefaultSSHConfig() string {
	if path, ok := os.LookupEnv("CONTAINERS_SSH_CONF"); ok {
		return path
	}
	dirname := homedir.Get()
	return filepath.Join(dirname, ".ssh", "config")
}

func useUserConfigLocations() bool {
	// NOTE: For now we want Windows to use system locations.
	// GetRootlessUID == -1 on Windows, so exclude negative range
	return unshare.GetRootlessUID() > 0
}
