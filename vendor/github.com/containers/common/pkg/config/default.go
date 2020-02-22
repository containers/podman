package config

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/containers/common/pkg/unshare"
	"github.com/containers/storage"
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
	DefaultInfraImage = "k8s.gcr.io/pause:3.1"
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
	DefaultApparmorProfile = "container-default"
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

	defaultLibpodConfig, err := defaultConfigFromMemory()
	if err != nil {
		return nil, err
	}

	var signaturePolicyPath string
	netns := "bridge"
	if unshare.IsRootless() {
		home, err := unshare.HomeDir()
		if err != nil {
			return nil, err
		}
		sigPath := filepath.Join(home, DefaultRootlessSignaturePolicyPath)
		if _, err := os.Stat(sigPath); err == nil {
			signaturePolicyPath = sigPath
		}
		netns = "slirp4netns"
	}

	return &Config{
		Containers: ContainersConfig{
			Devices:             []string{},
			Volumes:             []string{},
			Annotations:         []string{},
			ApparmorProfile:     DefaultApparmorProfile,
			CgroupNS:            "private",
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
			EnvHost:             false,
			HTTPProxy:           false,
			Init:                false,
			InitPath:            "",
			IPCNS:               "private",
			LogDriver:           DefaultLogDriver,
			LogSizeMax:          DefaultLogSizeMax,
			NetNS:               netns,
			NoHosts:             false,
			PidsLimit:           DefaultPidsLimit,
			PidNS:               "private",
			SeccompProfile:      SeccompDefaultPath,
			ShmSize:             DefaultShmSize,
			SignaturePolicyPath: signaturePolicyPath,
			UTSNS:               "private",
			UserNS:              "private",
			UserNSSize:          DefaultUserNSSize,
		},
		Network: NetworkConfig{
			DefaultNetwork:   "podman",
			NetworkConfigDir: cniConfigDir,
			CNIPluginDirs:    cniBinDir,
		},
		Libpod: *defaultLibpodConfig,
	}, nil
}

// defaultConfigFromMemory returns a default libpod configuration. Note that the
// config is different for root and rootless. It also parses the storage.conf.
func defaultConfigFromMemory() (*LibpodConfig, error) {
	c := new(LibpodConfig)
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
	c.StorageConfig = storeOpts

	c.HooksDir = DefaultHooksDirs
	c.ImageDefaultTransport = _defaultTransport
	c.StateType = BoltDBStateStore

	c.OCIRuntime = "runc"
	// If we're running on cgroups v2, default to using crun.
	if onCgroupsv2, _ := isCgroup2UnifiedMode(); onCgroupsv2 {
		c.OCIRuntime = "crun"
	}
	c.CgroupManager = SystemdCgroupsManager
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
	c.RuntimeSupportsJSON = []string{
		"crun",
		"runc",
	}
	c.RuntimeSupportsNoCgroups = []string{"crun"}
	c.InitPath = DefaultInitPath
	c.NoPivotRoot = false

	c.InfraCommand = DefaultInfraCommand
	c.InfraImage = DefaultInfraImage
	c.EnablePortReservation = true
	c.NumLocks = 2048
	c.EventsLogger = "journald"
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
