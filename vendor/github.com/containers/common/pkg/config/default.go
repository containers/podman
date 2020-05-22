package config

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/containers/common/pkg/apparmor"
	"github.com/containers/common/pkg/cgroupv2"
	"github.com/containers/common/pkg/sysinfo"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/unshare"
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
	DefaultInfraImage = "k8s.gcr.io/pause:3.2"
	// DefaultInfraCommand to be run in an infra container
	DefaultInfraCommand = "/pause"
	// DefaultRootlessSHMLockPath is the default path for rootless SHM locks
	DefaultRootlessSHMLockPath = "/libpod_rootless_lock"
	// DefaultDetachKeys is the default keys sequence for detaching a
	// container
	DefaultDetachKeys = "ctrl-p,ctrl-q"
)

var (
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
)

const (
	// EtcDir is the sysconfdir where podman should look for system config files.
	// It can be overridden at build time.
	_etcDir = "/etc"
	// InstallPrefix is the prefix where podman will be installed.
	// It can be overridden at build time.
	_installPrefix = "/usr"
	// CgroupfsCgroupsManager represents cgroupfs native cgroup manager
	CgroupfsCgroupsManager = "cgroupfs"
	// DefaultApparmorProfile  specifies the default apparmor profile for the container.
	DefaultApparmorProfile = apparmor.Profile
	// SystemdCgroupsManager represents systemd native cgroup manager
	SystemdCgroupsManager = "systemd"
	// DefaultLogDriver is the default type of log files
	DefaultLogDriver = "k8s-file"
	// DefaultLogSizeMax is the default value for the maximum log size
	// allowed for a container. Negative values mean that no limit is imposed.
	DefaultLogSizeMax = -1
	// DefaultPidsLimit is the default value for maximum number of processes
	// allowed inside a container
	DefaultPidsLimit = 2048
	// DefaultPullPolicy pulls the image if it does not exist locally
	DefaultPullPolicy = "missing"
	// DefaultSignaturePolicyPath is the default value for the
	// policy.json file.
	DefaultSignaturePolicyPath = "/etc/containers/policy.json"
	// DefaultRootlessSignaturePolicyPath is the default value for the
	// rootless policy.json file.
	DefaultRootlessSignaturePolicyPath = ".config/containers/policy.json"
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
)

// DefaultConfig defines the default values from containers.conf
func DefaultConfig() (*Config, error) {

	defaultEngineConfig, err := defaultConfigFromMemory()
	if err != nil {
		return nil, err
	}

	netns := "bridge"

	defaultEngineConfig.SignaturePolicyPath = DefaultSignaturePolicyPath
	if unshare.IsRootless() {
		home, err := unshare.HomeDir()
		if err != nil {
			return nil, err
		}
		sigPath := filepath.Join(home, DefaultRootlessSignaturePolicyPath)
		defaultEngineConfig.SignaturePolicyPath = sigPath
		if _, err := os.Stat(sigPath); err != nil {
			if _, err := os.Stat(DefaultSignaturePolicyPath); err == nil {
				defaultEngineConfig.SignaturePolicyPath = DefaultSignaturePolicyPath
			}
		}
		netns = "slirp4netns"
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
			CgroupNS:            cgroupNS,
			Cgroups:             "enabled",
			DefaultCapabilities: DefaultCapabilities,
			DefaultSysctls:      []string{},
			DefaultUlimits:      getDefaultProcessLimits(),
			DNSServers:          []string{},
			DNSOptions:          []string{},
			DNSSearches:         []string{},
			EnableLabeling:      selinuxEnabled(),
			Env: []string{
				"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			},
			EnvHost:        false,
			HTTPProxy:      false,
			Init:           false,
			InitPath:       "",
			IPCNS:          "private",
			LogDriver:      DefaultLogDriver,
			LogSizeMax:     DefaultLogSizeMax,
			NetNS:          netns,
			NoHosts:        false,
			PidsLimit:      DefaultPidsLimit,
			PidNS:          "private",
			SeccompProfile: SeccompDefaultPath,
			ShmSize:        DefaultShmSize,
			UTSNS:          "private",
			UserNS:         "host",
			UserNSSize:     DefaultUserNSSize,
		},
		Network: NetworkConfig{
			DefaultNetwork:   "podman",
			NetworkConfigDir: cniConfigDir,
			CNIPluginDirs:    cniBinDir,
		},
		Engine: *defaultEngineConfig,
	}, nil
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

	storeOpts, err := storage.DefaultStoreOptions(unshare.IsRootless(), unshare.GetRootlessUID())
	if err != nil {
		return nil, err
	}
	if storeOpts.GraphRoot == "" {
		logrus.Warnf("Storage configuration is unset - using hardcoded default graph root %q", _defaultGraphRoot)
		storeOpts.GraphRoot = _defaultGraphRoot
	}
	c.StaticDir = filepath.Join(storeOpts.GraphRoot, "libpod")
	c.VolumePath = filepath.Join(storeOpts.GraphRoot, "volumes")

	c.HooksDir = DefaultHooksDirs
	c.ImageDefaultTransport = _defaultTransport
	c.StateType = BoltDBStateStore

	c.OCIRuntime = "runc"
	// If we're running on cgroupv2 v2, default to using crun.
	if cgroup2, _ := cgroupv2.Enabled(); cgroup2 {
		c.OCIRuntime = "crun"
	}
	c.CgroupManager = defaultCgroupManager()
	c.StopTimeout = uint(10)

	c.OCIRuntimes = map[string][]string{
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
		"crun": {
			"/usr/bin/crun",
			"/usr/sbin/crun",
			"/usr/local/bin/crun",
			"/usr/local/sbin/crun",
			"/sbin/crun",
			"/bin/crun",
			"/run/current-system/sw/bin/crun",
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
	}
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
	}
	c.RuntimeSupportsNoCgroups = []string{"crun"}
	c.RuntimeSupportsKVM = []string{"kata", "kata-runtime", "kata-qemu", "kata-fc"}
	c.InitPath = DefaultInitPath
	c.NoPivotRoot = false

	c.InfraCommand = DefaultInfraCommand
	c.InfraImage = DefaultInfraImage
	c.EnablePortReservation = true
	c.NumLocks = 2048
	c.EventsLogger = defaultEventsLogger()
	c.DetachKeys = DefaultDetachKeys
	c.SDNotify = false
	// TODO - ideally we should expose a `type LockType string` along with
	// constants.
	c.LockType = "shm"

	return c, nil
}

func defaultTmpDir() (string, error) {
	if !unshare.IsRootless() {
		return "/var/run/libpod", nil
	}

	runtimeDir, err := getRuntimeDir()
	if err != nil {
		return "", err
	}
	libpodRuntimeDir := filepath.Join(runtimeDir, "libpod")

	if err := os.Mkdir(libpodRuntimeDir, 0700|os.ModeSticky); err != nil {
		if !os.IsExist(err) {
			return "", errors.Wrapf(err, "cannot mkdir %s", libpodRuntimeDir)
		} else if err := os.Chmod(libpodRuntimeDir, 0700|os.ModeSticky); err != nil {
			// The directory already exist, just set the sticky bit
			return "", errors.Wrapf(err, "could not set sticky bit on %s", libpodRuntimeDir)
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
	err := cmd.Run()
	if err != nil {
		return err
	}
	r := regexp.MustCompile(`^conmon version (?P<Major>\d+).(?P<Minor>\d+).(?P<Patch>\d+)`)

	matches := r.FindStringSubmatch(out.String())
	if len(matches) != 4 {
		return errors.Wrap(err, _conmonVersionFormatErr)
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
	if c.Containers.NetNS == "private" && unshare.IsRootless() {
		return "slirp4netns"
	}
	return c.Containers.NetNS
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
		if c.Engine.CgroupManager == SystemdCgroupsManager {
			cgroup2, _ := cgroupv2.Enabled()
			if cgroup2 {
				return c.Containers.PidsLimit
			}
			return 0
		}
	}
	return sysinfo.GetDefaultPidsLimit()
}

// DetachKeys returns the default detach keys to detach from a container
func (c *Config) DetachKeys() string {
	return c.Engine.DetachKeys
}
