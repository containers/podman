package config

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	nettypes "github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/apparmor"
	"github.com/containers/common/pkg/cgroupv2"
	"github.com/containers/common/pkg/util"
	"github.com/containers/storage/pkg/homedir"
	"github.com/containers/storage/pkg/unshare"
	"github.com/containers/storage/types"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// _conmonMinMajorVersion is the major version required for conmon.
	_conmonMinMajorVersion = 2

	// _conmonMinMinorVersion is the minor version required for conmon.
	_conmonMinMinorVersion = 0

	// _conmonMinPatchVersion is the sub-minor version required for conmon.
	_conmonMinPatchVersion = 1

	// _conmonVersionFormatErr is used when the expected versio-format of conmon
	// has changed.
	_conmonVersionFormatErr = "conmon version changed format"

	// _defaultGraphRoot points to the default path of the graph root.
	_defaultGraphRoot = "/var/lib/containers/storage"

	// _defaultTransport is a prefix that we apply to an image name to check
	// docker hub first for the image.
	_defaultTransport = "docker://"
)

var (
	// DefaultInitPath is the default path to the container-init binary
	DefaultInitPath = "/usr/libexec/podman/catatonit"
	// DefaultInfraImage to use for infra container
	DefaultInfraImage = ""
	// DefaultRootlessSHMLockPath is the default path for rootless SHM locks
	DefaultRootlessSHMLockPath = "/libpod_rootless_lock"
	// DefaultDetachKeys is the default keys sequence for detaching a
	// container
	DefaultDetachKeys = "ctrl-p,ctrl-q"
	// ErrConmonOutdated indicates the version of conmon found (whether via the configuration or $PATH)
	// is out of date for the current podman version
	ErrConmonOutdated = errors.New("outdated conmon version")
	// ErrInvalidArg indicates that an invalid argument was passed
	ErrInvalidArg = errors.New("invalid argument")
	// DefaultHooksDirs defines the default hooks directory
	DefaultHooksDirs = []string{"/usr/share/containers/oci/hooks.d"}
	// DefaultCapabilities for the default_capabilities option in the containers.conf file
	DefaultCapabilities = []string{
		"CAP_AUDIT_WRITE",
		"CAP_CHOWN",
		"CAP_DAC_OVERRIDE",
		"CAP_FOWNER",
		"CAP_FSETID",
		"CAP_KILL",
		"CAP_MKNOD",
		"CAP_NET_BIND_SERVICE",
		"CAP_NET_RAW",
		"CAP_SETFCAP",
		"CAP_SETGID",
		"CAP_SETPCAP",
		"CAP_SETUID",
		"CAP_SYS_CHROOT",
	}

	// It may seem a bit unconventional, but it is necessary to do so
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
	// helper binary in a different location
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
	// CgroupfsCgroupsManager represents cgroupfs native cgroup manager
	CgroupfsCgroupsManager = "cgroupfs"
	// DefaultApparmorProfile  specifies the default apparmor profile for the container.
	DefaultApparmorProfile = apparmor.Profile
	// DefaultHostsFile is the default path to the hosts file
	DefaultHostsFile = "/etc/hosts"
	// SystemdCgroupsManager represents systemd native cgroup manager
	SystemdCgroupsManager = "systemd"
	// DefaultLogSizeMax is the default value for the maximum log size
	// allowed for a container. Negative values mean that no limit is imposed.
	DefaultLogSizeMax = -1
	// DefaultEventsLogSize is the default value for the maximum events log size
	// before rotation.
	DefaultEventsLogSizeMax = uint64(1000000)
	// DefaultPidsLimit is the default value for maximum number of processes
	// allowed inside a container
	DefaultPidsLimit = 2048
	// DefaultPullPolicy pulls the image if it does not exist locally
	DefaultPullPolicy = "missing"
	// DefaultSignaturePolicyPath is the default value for the
	// policy.json file.
	DefaultSignaturePolicyPath = "/etc/containers/policy.json"
	// DefaultSubnet is the subnet that will be used for the default
	// network.
	DefaultSubnet = "10.88.0.0/16"
	// DefaultRootlessSignaturePolicyPath is the location within
	// XDG_CONFIG_HOME of the rootless policy.json file.
	DefaultRootlessSignaturePolicyPath = "containers/policy.json"
	// DefaultShmSize default value
	DefaultShmSize = "65536k"
	// DefaultUserNSSize default value
	DefaultUserNSSize = 65536
	// OCIBufSize limits maximum LogSizeMax
	OCIBufSize = 8192
	// SeccompOverridePath if this exists it overrides the default seccomp path.
	SeccompOverridePath = _etcDir + "/containers/seccomp.json"
	// SeccompDefaultPath defines the default seccomp path.
	SeccompDefaultPath = _installPrefix + "/share/containers/seccomp.json"
	// DefaultVolumePluginTimeout is the default volume plugin timeout, in seconds
	DefaultVolumePluginTimeout = 5
)

// DefaultConfig defines the default values from containers.conf
func DefaultConfig() (*Config, error) {
	defaultEngineConfig, err := defaultConfigFromMemory()
	if err != nil {
		return nil, err
	}

	defaultEngineConfig.SignaturePolicyPath = DefaultSignaturePolicyPath
	if unshare.IsRootless() {
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
			Cgroups:             "enabled",
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
			UserNSSize: DefaultUserNSSize,
		},
		Network: NetworkConfig{
			DefaultNetwork:     "podman",
			DefaultSubnet:      DefaultSubnet,
			DefaultSubnetPools: DefaultSubnetPools,
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
		Volumes:  []string{"$HOME:$HOME"},
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

	c.EventsLogFilePath = filepath.Join(c.TmpDir, "events", "events.log")

	c.EventsLogFileMaxSize = eventsLogMaxSize(DefaultEventsLogSizeMax)

	c.CompatAPIEnforceDockerHub = true

	if path, ok := os.LookupEnv("CONTAINERS_STORAGE_CONF"); ok {
		types.SetDefaultConfigFilePath(path)
	}
	storeOpts, err := types.DefaultStoreOptions(unshare.IsRootless(), unshare.GetRootlessUID())
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
		"krun": {
			"/usr/bin/krun",
			"/usr/local/bin/krun",
		},
	}
	// Needs to be called after populating c.OCIRuntimes
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
	c.PullPolicy = DefaultPullPolicy
	c.RuntimeSupportsJSON = []string{
		"crun",
		"runc",
		"kata",
		"runsc",
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
	c.LockType = "shm"
	c.MachineEnabled = false
	c.ChownCopiedFiles = true

	c.PodExitPolicy = defaultPodExitPolicy

	return c, nil
}

func defaultTmpDir() (string, error) {
	if !unshare.IsRootless() {
		return "/run/libpod", nil
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
			// The directory already exist, just set the sticky bit
			return "", errors.Wrap(err, "set sticky bit on")
		}
	}
	return filepath.Join(libpodRuntimeDir, "tmp"), nil
}

// probeConmon calls conmon --version and verifies it is a new enough version for
// the runtime expectations the container engine currently has.
func probeConmon(conmonBinary string) error {
	cmd := exec.Command(conmonBinary, "--version")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return err
	}
	r := regexp.MustCompile(`^conmon version (?P<Major>\d+).(?P<Minor>\d+).(?P<Patch>\d+)`)

	matches := r.FindStringSubmatch(out.String())
	if len(matches) != 4 {
		return errors.New(_conmonVersionFormatErr)
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return errors.Wrap(err, _conmonVersionFormatErr)
	}
	if major < _conmonMinMajorVersion {
		return ErrConmonOutdated
	}
	if major > _conmonMinMajorVersion {
		return nil
	}

	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return errors.Wrap(err, _conmonVersionFormatErr)
	}
	if minor < _conmonMinMinorVersion {
		return ErrConmonOutdated
	}
	if minor > _conmonMinMinorVersion {
		return nil
	}

	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return errors.Wrap(err, _conmonVersionFormatErr)
	}
	if patch < _conmonMinPatchVersion {
		return ErrConmonOutdated
	}
	if patch > _conmonMinPatchVersion {
		return nil
	}

	return nil
}

// NetNS returns the default network namespace
func (c *Config) NetNS() string {
	return c.Containers.NetNS
}

func (c EngineConfig) EventsLogMaxSize() uint64 {
	return uint64(c.EventsLogFileMaxSize)
}

// SecurityOptions returns the default security options
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

// Sysctls returns the default sysctls
func (c *Config) Sysctls() []string {
	return c.Containers.DefaultSysctls
}

// Volumes returns the default additional volumes for containersvolumes
func (c *Config) Volumes() []string {
	return c.Containers.Volumes
}

// Devices returns the default additional devices for containers
func (c *Config) Devices() []string {
	return c.Containers.Devices
}

// DNSServers returns the default DNS servers to add to resolv.conf in containers
func (c *Config) DNSServers() []string {
	return c.Containers.DNSServers
}

// DNSSerches returns the default DNS searches to add to resolv.conf in containers
func (c *Config) DNSSearches() []string {
	return c.Containers.DNSSearches
}

// DNSOptions returns the default DNS options to add to resolv.conf in containers
func (c *Config) DNSOptions() []string {
	return c.Containers.DNSOptions
}

// Env returns the default additional environment variables to add to containers
func (c *Config) Env() []string {
	return c.Containers.Env
}

// InitPath returns the default init path to add to containers
func (c *Config) InitPath() string {
	return c.Containers.InitPath
}

// IPCNS returns the default IPC Namespace configuration to run containers with
func (c *Config) IPCNS() string {
	return c.Containers.IPCNS
}

// PIDNS returns the default PID Namespace configuration to run containers with
func (c *Config) PidNS() string {
	return c.Containers.PidNS
}

// CgroupNS returns the default Cgroup Namespace configuration to run containers with
func (c *Config) CgroupNS() string {
	return c.Containers.CgroupNS
}

// Cgroups returns whether to containers with cgroup confinement
func (c *Config) Cgroups() string {
	return c.Containers.Cgroups
}

// UTSNS returns the default UTS Namespace configuration to run containers with
func (c *Config) UTSNS() string {
	return c.Containers.UTSNS
}

// ShmSize returns the default size for temporary file systems to use in containers
func (c *Config) ShmSize() string {
	return c.Containers.ShmSize
}

// Ulimits returns the default ulimits to use in containers
func (c *Config) Ulimits() []string {
	return c.Containers.DefaultUlimits
}

// PidsLimit returns the default maximum number of pids to use in containers
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

// DetachKeys returns the default detach keys to detach from a container
func (c *Config) DetachKeys() string {
	return c.Engine.DetachKeys
}

// Tz returns the timezone in the container
func (c *Config) TZ() string {
	return c.Containers.TZ
}

func (c *Config) Umask() string {
	return c.Containers.Umask
}

// LogDriver returns the logging driver to be used
// currently k8s-file or journald
func (c *Config) LogDriver() string {
	return c.Containers.LogDriver
}

// MachineEnabled returns if podman is running inside a VM or not
func (c *Config) MachineEnabled() bool {
	return c.Engine.MachineEnabled
}

// MachineVolumes returns volumes to mount into the VM
func (c *Config) MachineVolumes() ([]string, error) {
	return machineVolumes(c.Machine.Volumes)
}

func machineVolumes(volumes []string) ([]string, error) {
	translatedVolumes := []string{}
	for _, v := range volumes {
		vol := os.ExpandEnv(v)
		split := strings.Split(vol, ":")
		if len(split) < 2 || len(split) > 3 {
			return nil, errors.Errorf("invalid machine volume %s, 2 or 3 fields required", v)
		}
		if split[0] == "" || split[1] == "" {
			return nil, errors.Errorf("invalid machine volume %s, fields must container data", v)
		}
		translatedVolumes = append(translatedVolumes, vol)
	}
	return translatedVolumes, nil
}
