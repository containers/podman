package config

import (
	"os"
	"path/filepath"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/libpod/events"
	"github.com/containers/libpod/pkg/cgroups"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// _defaultGraphRoot points to the default path of the graph root.
	_defaultGraphRoot = "/var/lib/containers/storage"
	// _defaultRootlessSignaturePolicyPath points to the default path of the
	// rootless policy.json file.
	_defaultRootlessSignaturePolicyPath = ".config/containers/policy.json"
)

// defaultConfigFromMemory returns a default libpod configuration. Note that the
// config is different for root and rootless. It also parses the storage.conf.
func defaultConfigFromMemory() (*Config, error) {
	c := new(Config)
	if tmp, err := defaultTmpDir(); err != nil {
		return nil, err
	} else {
		c.TmpDir = tmp
	}
	c.EventsLogFilePath = filepath.Join(c.TmpDir, "events", "events.log")

	storeOpts, err := storage.DefaultStoreOptions(rootless.IsRootless(), rootless.GetRootlessUID())
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

	c.ImageDefaultTransport = _defaultTransport
	c.StateType = define.BoltDBStateStore
	c.OCIRuntime = "runc"

	// If we're running on cgroups v2, default to using crun.
	if onCgroupsv2, _ := cgroups.IsCgroup2UnifiedMode(); onCgroupsv2 {
		c.OCIRuntime = "crun"
	}

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
	c.ConmonEnvVars = []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	}
	c.RuntimeSupportsJSON = []string{
		"crun",
		"runc",
	}
	c.RuntimeSupportsNoCgroups = []string{"crun"}
	c.InitPath = define.DefaultInitPath
	c.CgroupManager = define.SystemdCgroupsManager
	c.MaxLogSize = -1
	c.NoPivotRoot = false
	c.CNIConfigDir = _etcDir + "/cni/net.d/"
	c.CNIPluginDir = []string{
		"/usr/libexec/cni",
		"/usr/lib/cni",
		"/usr/local/lib/cni",
		"/opt/cni/bin",
	}
	c.CNIDefaultNetwork = "podman"
	c.InfraCommand = define.DefaultInfraCommand
	c.InfraImage = define.DefaultInfraImage
	c.EnablePortReservation = true
	c.EnableLabeling = true
	c.NumLocks = 2048
	c.EventsLogger = events.DefaultEventerType.String()
	c.DetachKeys = define.DefaultDetachKeys
	// TODO - ideally we should expose a `type LockType string` along with
	// constants.
	c.LockType = "shm"

	if rootless.IsRootless() {
		home, err := util.HomeDir()
		if err != nil {
			return nil, err
		}
		sigPath := filepath.Join(home, _defaultRootlessSignaturePolicyPath)
		if _, err := os.Stat(sigPath); err == nil {
			c.SignaturePolicyPath = sigPath
		}
	}
	return c, nil
}

func defaultTmpDir() (string, error) {
	if !rootless.IsRootless() {
		return "/var/run/libpod", nil
	}

	runtimeDir, err := util.GetRuntimeDir()
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
