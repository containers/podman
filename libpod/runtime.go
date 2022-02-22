package libpod

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/common/libimage"
	"github.com/containers/common/libnetwork/network"
	nettypes "github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/secrets"
	"github.com/containers/image/v5/pkg/sysregistriesv2"
	is "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/libpod/lock"
	"github.com/containers/podman/v4/libpod/plugin"
	"github.com/containers/podman/v4/libpod/shutdown"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/systemd"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/podman/v4/utils"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/unshare"
	"github.com/docker/docker/pkg/namesgenerator"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// conmonMinMajorVersion is the major version required for conmon.
	conmonMinMajorVersion = 2

	// conmonMinMinorVersion is the minor version required for conmon.
	conmonMinMinorVersion = 0

	// conmonMinPatchVersion is the sub-minor version required for conmon.
	conmonMinPatchVersion = 24
)

// A RuntimeOption is a functional option which alters the Runtime created by
// NewRuntime
type RuntimeOption func(*Runtime) error

type storageSet struct {
	RunRootSet         bool
	GraphRootSet       bool
	StaticDirSet       bool
	VolumePathSet      bool
	GraphDriverNameSet bool
	TmpDirSet          bool
}

// Runtime is the core libpod runtime
type Runtime struct {
	config        *config.Config
	storageConfig storage.StoreOptions
	storageSet    storageSet

	state                  State
	store                  storage.Store
	storageService         *storageService
	imageContext           *types.SystemContext
	defaultOCIRuntime      OCIRuntime
	ociRuntimes            map[string]OCIRuntime
	runtimeFlags           []string
	network                nettypes.ContainerNetwork
	conmonPath             string
	libimageRuntime        *libimage.Runtime
	libimageEventsShutdown chan bool
	lockManager            lock.Manager

	// syslog describes whenever logrus should log to the syslog as well.
	// Note that the syslog hook will be enabled early in cmd/podman/syslog_linux.go
	// This bool is just needed so that we can set it for netavark interface.
	syslog bool

	// doRenumber indicates that the runtime should perform a lock renumber
	// during initialization.
	// Once the runtime has been initialized and returned, this variable is
	// unused.
	doRenumber bool

	doMigrate bool
	// System migrate can move containers to a new runtime.
	// We make no promises that these migrated containers work on the new
	// runtime, though.
	migrateRuntime string

	// valid indicates whether the runtime is ready to use.
	// valid is set to true when a runtime is returned from GetRuntime(),
	// and remains true until the runtime is shut down (rendering its
	// storage unusable). When valid is false, the runtime cannot be used.
	valid bool

	// mechanism to read and write even logs
	eventer events.Eventer

	// noStore indicates whether we need to interact with a store or not
	noStore bool
	// secretsManager manages secrets
	secretsManager *secrets.SecretsManager
}

// SetXdgDirs ensures the XDG_RUNTIME_DIR env and XDG_CONFIG_HOME variables are set.
// containers/image uses XDG_RUNTIME_DIR to locate the auth file, XDG_CONFIG_HOME is
// use for the containers.conf configuration file.
func SetXdgDirs() error {
	if !rootless.IsRootless() {
		return nil
	}

	// Setup XDG_RUNTIME_DIR
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")

	if runtimeDir == "" {
		var err error
		runtimeDir, err = util.GetRuntimeDir()
		if err != nil {
			return err
		}
	}
	if err := os.Setenv("XDG_RUNTIME_DIR", runtimeDir); err != nil {
		return errors.Wrapf(err, "cannot set XDG_RUNTIME_DIR")
	}

	if rootless.IsRootless() && os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		sessionAddr := filepath.Join(runtimeDir, "bus")
		if _, err := os.Stat(sessionAddr); err == nil {
			os.Setenv("DBUS_SESSION_BUS_ADDRESS", fmt.Sprintf("unix:path=%s", sessionAddr))
		}
	}

	// Setup XDG_CONFIG_HOME
	if cfgHomeDir := os.Getenv("XDG_CONFIG_HOME"); cfgHomeDir == "" {
		cfgHomeDir, err := util.GetRootlessConfigHomeDir()
		if err != nil {
			return err
		}
		if err := os.Setenv("XDG_CONFIG_HOME", cfgHomeDir); err != nil {
			return errors.Wrapf(err, "cannot set XDG_CONFIG_HOME")
		}
	}
	return nil
}

// NewRuntime creates a new container runtime
// Options can be passed to override the default configuration for the runtime
func NewRuntime(ctx context.Context, options ...RuntimeOption) (*Runtime, error) {
	conf, err := config.NewConfig("")
	if err != nil {
		return nil, err
	}
	return newRuntimeFromConfig(ctx, conf, options...)
}

// NewRuntimeFromConfig creates a new container runtime using the given
// configuration file for its default configuration. Passed RuntimeOption
// functions can be used to mutate this configuration further.
// An error will be returned if the configuration file at the given path does
// not exist or cannot be loaded
func NewRuntimeFromConfig(ctx context.Context, userConfig *config.Config, options ...RuntimeOption) (*Runtime, error) {
	return newRuntimeFromConfig(ctx, userConfig, options...)
}

func newRuntimeFromConfig(ctx context.Context, conf *config.Config, options ...RuntimeOption) (*Runtime, error) {
	runtime := new(Runtime)

	if conf.Engine.OCIRuntime == "" {
		conf.Engine.OCIRuntime = "runc"
		// If we're running on cgroups v2, default to using crun.
		if onCgroupsv2, _ := cgroups.IsCgroup2UnifiedMode(); onCgroupsv2 {
			conf.Engine.OCIRuntime = "crun"
		}
	}

	runtime.config = conf

	if err := SetXdgDirs(); err != nil {
		return nil, err
	}

	storeOpts, err := storage.DefaultStoreOptions(rootless.IsRootless(), rootless.GetRootlessUID())
	if err != nil {
		return nil, err
	}
	runtime.storageConfig = storeOpts

	// Overwrite config with user-given configuration options
	for _, opt := range options {
		if err := opt(runtime); err != nil {
			return nil, errors.Wrapf(err, "error configuring runtime")
		}
	}

	if err := shutdown.Register("libpod", func(sig os.Signal) error {
		os.Exit(1)
		return nil
	}); err != nil && errors.Cause(err) != shutdown.ErrHandlerExists {
		logrus.Errorf("Registering shutdown handler for libpod: %v", err)
	}

	if err := shutdown.Start(); err != nil {
		return nil, errors.Wrapf(err, "error starting shutdown signal handler")
	}

	if err := makeRuntime(ctx, runtime); err != nil {
		return nil, err
	}

	runtime.config.CheckCgroupsAndAdjustConfig()

	return runtime, nil
}

func getLockManager(runtime *Runtime) (lock.Manager, error) {
	var err error
	var manager lock.Manager

	switch runtime.config.Engine.LockType {
	case "file":
		lockPath := filepath.Join(runtime.config.Engine.TmpDir, "locks")
		manager, err = lock.OpenFileLockManager(lockPath)
		if err != nil {
			if os.IsNotExist(errors.Cause(err)) {
				manager, err = lock.NewFileLockManager(lockPath)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to get new file lock manager")
				}
			} else {
				return nil, err
			}
		}

	case "", "shm":
		lockPath := define.DefaultSHMLockPath
		if rootless.IsRootless() {
			lockPath = fmt.Sprintf("%s_%d", define.DefaultRootlessSHMLockPath, rootless.GetRootlessUID())
		}
		// Set up the lock manager
		manager, err = lock.OpenSHMLockManager(lockPath, runtime.config.Engine.NumLocks)
		if err != nil {
			switch {
			case os.IsNotExist(errors.Cause(err)):
				manager, err = lock.NewSHMLockManager(lockPath, runtime.config.Engine.NumLocks)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to get new shm lock manager")
				}
			case errors.Cause(err) == syscall.ERANGE && runtime.doRenumber:
				logrus.Debugf("Number of locks does not match - removing old locks")

				// ERANGE indicates a lock numbering mismatch.
				// Since we're renumbering, this is not fatal.
				// Remove the earlier set of locks and recreate.
				if err := os.Remove(filepath.Join("/dev/shm", lockPath)); err != nil {
					return nil, errors.Wrapf(err, "error removing libpod locks file %s", lockPath)
				}

				manager, err = lock.NewSHMLockManager(lockPath, runtime.config.Engine.NumLocks)
				if err != nil {
					return nil, err
				}
			default:
				return nil, err
			}
		}
	default:
		return nil, errors.Wrapf(define.ErrInvalidArg, "unknown lock type %s", runtime.config.Engine.LockType)
	}
	return manager, nil
}

// Make a new runtime based on the given configuration
// Sets up containers/storage, state store, OCI runtime
func makeRuntime(ctx context.Context, runtime *Runtime) (retErr error) {
	// Find a working conmon binary
	cPath, err := findConmon(runtime.config.Engine.ConmonPath)
	if err != nil {
		return err
	}
	runtime.conmonPath = cPath

	// Make the static files directory if it does not exist
	if err := os.MkdirAll(runtime.config.Engine.StaticDir, 0700); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return errors.Wrap(err, "error creating runtime static files directory")
		}
	}

	// Set up the state.
	//
	// TODO - if we further break out the state implementation into
	// libpod/state, the config could take care of the code below.  It
	// would further allow to move the types and consts into a coherent
	// package.
	switch runtime.config.Engine.StateType {
	case config.InMemoryStateStore:
		return errors.Wrapf(define.ErrInvalidArg, "in-memory state is currently disabled")
	case config.SQLiteStateStore:
		return errors.Wrapf(define.ErrInvalidArg, "SQLite state is currently disabled")
	case config.BoltDBStateStore:
		dbPath := filepath.Join(runtime.config.Engine.StaticDir, "bolt_state.db")

		state, err := NewBoltState(dbPath, runtime)
		if err != nil {
			return err
		}
		runtime.state = state
	default:
		return errors.Wrapf(define.ErrInvalidArg, "unrecognized state type passed (%v)", runtime.config.Engine.StateType)
	}

	// Grab config from the database so we can reset some defaults
	dbConfig, err := runtime.state.GetDBConfig()
	if err != nil {
		return errors.Wrapf(err, "error retrieving runtime configuration from database")
	}

	runtime.mergeDBConfig(dbConfig)

	unified, _ := cgroups.IsCgroup2UnifiedMode()
	if unified && rootless.IsRootless() && !systemd.IsSystemdSessionValid(rootless.GetRootlessUID()) {
		// If user is rootless and XDG_RUNTIME_DIR is found, podman will not proceed with /tmp directory
		// it will try to use existing XDG_RUNTIME_DIR
		// if current user has no write access to XDG_RUNTIME_DIR we will fail later
		if err := unix.Access(runtime.storageConfig.RunRoot, unix.W_OK); err != nil {
			msg := "XDG_RUNTIME_DIR is pointing to a path which is not writable. Most likely podman will fail."
			if errors.Is(err, os.ErrNotExist) {
				// if dir does not exists try to create it
				if err := os.MkdirAll(runtime.storageConfig.RunRoot, 0700); err != nil {
					logrus.Warn(msg)
				}
			} else {
				logrus.Warnf("%s: %v", msg, err)
			}
		}
	}

	logrus.Debugf("Using graph driver %s", runtime.storageConfig.GraphDriverName)
	logrus.Debugf("Using graph root %s", runtime.storageConfig.GraphRoot)
	logrus.Debugf("Using run root %s", runtime.storageConfig.RunRoot)
	logrus.Debugf("Using static dir %s", runtime.config.Engine.StaticDir)
	logrus.Debugf("Using tmp dir %s", runtime.config.Engine.TmpDir)
	logrus.Debugf("Using volume path %s", runtime.config.Engine.VolumePath)

	// Validate our config against the database, now that we've set our
	// final storage configuration
	if err := runtime.state.ValidateDBConfig(runtime); err != nil {
		return err
	}

	if err := runtime.state.SetNamespace(runtime.config.Engine.Namespace); err != nil {
		return errors.Wrapf(err, "error setting libpod namespace in state")
	}
	logrus.Debugf("Set libpod namespace to %q", runtime.config.Engine.Namespace)

	hasCapSysAdmin, err := unshare.HasCapSysAdmin()
	if err != nil {
		return err
	}

	needsUserns := !hasCapSysAdmin

	// Set up containers/storage
	var store storage.Store
	if needsUserns {
		logrus.Debug("Not configuring container store")
	} else if runtime.noStore {
		logrus.Debug("No store required. Not opening container store.")
	} else if err := runtime.configureStore(); err != nil {
		return err
	}
	defer func() {
		if retErr != nil && store != nil {
			// Don't forcibly shut down
			// We could be opening a store in use by another libpod
			if _, err := store.Shutdown(false); err != nil {
				logrus.Errorf("Removing store for partially-created runtime: %s", err)
			}
		}
	}()

	// Setup the eventer
	eventer, err := runtime.newEventer()
	if err != nil {
		return err
	}
	runtime.eventer = eventer
	// TODO: events for libimage

	// Set up containers/image
	if runtime.imageContext == nil {
		runtime.imageContext = &types.SystemContext{
			BigFilesTemporaryDir: parse.GetTempDir(),
		}
	}
	runtime.imageContext.SignaturePolicyPath = runtime.config.Engine.SignaturePolicyPath

	// Create the tmpDir
	if err := os.MkdirAll(runtime.config.Engine.TmpDir, 0751); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return errors.Wrap(err, "error creating tmpdir")
		}
	}

	// Create events log dir
	if err := os.MkdirAll(filepath.Dir(runtime.config.Engine.EventsLogFilePath), 0700); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return errors.Wrap(err, "error creating events dirs")
		}
	}

	// Get us at least one working OCI runtime.
	runtime.ociRuntimes = make(map[string]OCIRuntime)

	// Initialize remaining OCI runtimes
	for name, paths := range runtime.config.Engine.OCIRuntimes {
		ociRuntime, err := newConmonOCIRuntime(name, paths, runtime.conmonPath, runtime.runtimeFlags, runtime.config)
		if err != nil {
			// Don't fatally error.
			// This will allow us to ship configs including optional
			// runtimes that might not be installed (crun, kata).
			// Only a infof so default configs don't spec errors.
			logrus.Debugf("Configured OCI runtime %s initialization failed: %v", name, err)
			continue
		}

		runtime.ociRuntimes[name] = ociRuntime
	}

	// Do we have a default OCI runtime?
	if runtime.config.Engine.OCIRuntime != "" {
		// If the string starts with / it's a path to a runtime
		// executable.
		if strings.HasPrefix(runtime.config.Engine.OCIRuntime, "/") {
			ociRuntime, err := newConmonOCIRuntime(runtime.config.Engine.OCIRuntime, []string{runtime.config.Engine.OCIRuntime}, runtime.conmonPath, runtime.runtimeFlags, runtime.config)
			if err != nil {
				return err
			}

			runtime.ociRuntimes[runtime.config.Engine.OCIRuntime] = ociRuntime
			runtime.defaultOCIRuntime = ociRuntime
		} else {
			ociRuntime, ok := runtime.ociRuntimes[runtime.config.Engine.OCIRuntime]
			if !ok {
				return errors.Wrapf(define.ErrInvalidArg, "default OCI runtime %q not found", runtime.config.Engine.OCIRuntime)
			}
			runtime.defaultOCIRuntime = ociRuntime
		}
	}
	logrus.Debugf("Using OCI runtime %q", runtime.defaultOCIRuntime.Path())

	// Do we have at least one valid OCI runtime?
	if len(runtime.ociRuntimes) == 0 {
		return errors.Wrapf(define.ErrInvalidArg, "no OCI runtime has been configured")
	}

	// Do we have a default runtime?
	if runtime.defaultOCIRuntime == nil {
		return errors.Wrapf(define.ErrInvalidArg, "no default OCI runtime was configured")
	}

	// Make the per-boot files directory if it does not exist
	if err := os.MkdirAll(runtime.config.Engine.TmpDir, 0755); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return errors.Wrapf(err, "error creating runtime temporary files directory")
		}
	}

	// the store is only setup when we are in the userns so we do the same for the network interface
	if !needsUserns {
		netBackend, netInterface, err := network.NetworkBackend(runtime.store, runtime.config, runtime.syslog)
		if err != nil {
			return err
		}
		runtime.config.Network.NetworkBackend = string(netBackend)
		runtime.network = netInterface
	}

	// We now need to see if the system has restarted
	// We check for the presence of a file in our tmp directory to verify this
	// This check must be locked to prevent races
	runtimeAliveLock := filepath.Join(runtime.config.Engine.TmpDir, "alive.lck")
	runtimeAliveFile := filepath.Join(runtime.config.Engine.TmpDir, "alive")
	aliveLock, err := storage.GetLockfile(runtimeAliveLock)
	if err != nil {
		return errors.Wrapf(err, "error acquiring runtime init lock")
	}
	// Acquire the lock and hold it until we return
	// This ensures that no two processes will be in runtime.refresh at once
	// TODO: we can't close the FD in this lock, so we should keep it around
	// and use it to lock important operations
	aliveLock.Lock()
	doRefresh := false
	defer func() {
		if aliveLock.Locked() {
			aliveLock.Unlock()
		}
	}()

	_, err = os.Stat(runtimeAliveFile)
	if err != nil {
		// If we need to refresh, then it is safe to assume there are
		// no containers running.  Create immediately a namespace, as
		// we will need to access the storage.
		if needsUserns {
			// warn users if mode is rootless and cgroup manager is systemd
			// and no valid systemd session is present
			// warn only whenever new namespace is created
			if runtime.config.Engine.CgroupManager == config.SystemdCgroupsManager {
				unified, _ := cgroups.IsCgroup2UnifiedMode()
				if unified && rootless.IsRootless() && !systemd.IsSystemdSessionValid(rootless.GetRootlessUID()) {
					logrus.Debug("Invalid systemd user session for current user")
				}
			}
			aliveLock.Unlock() // Unlock to avoid deadlock as BecomeRootInUserNS will reexec.
			pausePid, err := util.GetRootlessPauseProcessPidPathGivenDir(runtime.config.Engine.TmpDir)
			if err != nil {
				return errors.Wrapf(err, "could not get pause process pid file path")
			}
			became, ret, err := rootless.BecomeRootInUserNS(pausePid)
			if err != nil {
				return err
			}
			if became {
				// Check if the pause process was created.  If it was created, then
				// move it to its own systemd scope.
				utils.MovePauseProcessToScope(pausePid)
				os.Exit(ret)
			}
		}
		// If the file doesn't exist, we need to refresh the state
		// This will trigger on first use as well, but refreshing an
		// empty state only creates a single file
		// As such, it's not really a performance concern
		if os.IsNotExist(err) {
			doRefresh = true
		} else {
			return errors.Wrapf(err, "error reading runtime status file %s", runtimeAliveFile)
		}
	}

	runtime.lockManager, err = getLockManager(runtime)
	if err != nil {
		return err
	}

	// If we're renumbering locks, do it now.
	// It breaks out of normal runtime init, and will not return a valid
	// runtime.
	if runtime.doRenumber {
		if err := runtime.renumberLocks(); err != nil {
			return err
		}
	}

	// If we need to refresh the state, do it now - things are guaranteed to
	// be set up by now.
	if doRefresh {
		// Ensure we have a store before refresh occurs
		if runtime.store == nil {
			if err := runtime.configureStore(); err != nil {
				return err
			}
		}

		if err2 := runtime.refresh(runtimeAliveFile); err2 != nil {
			return err2
		}
	}

	// Mark the runtime as valid - ready to be used, cannot be modified
	// further
	runtime.valid = true

	if runtime.doMigrate {
		if err := runtime.migrate(ctx); err != nil {
			return err
		}
	}

	return nil
}

// findConmon iterates over conmonPaths and returns the path
// to the first conmon binary with a new enough version. If none is found,
// we try to do a path lookup of "conmon".
func findConmon(conmonPaths []string) (string, error) {
	foundOutdatedConmon := false
	for _, path := range conmonPaths {
		stat, err := os.Stat(path)
		if err != nil {
			continue
		}
		if stat.IsDir() {
			continue
		}
		if err := probeConmon(path); err != nil {
			logrus.Warnf("Conmon at %s invalid: %v", path, err)
			foundOutdatedConmon = true
			continue
		}
		logrus.Debugf("Using conmon: %q", path)
		return path, nil
	}

	// Search the $PATH as last fallback
	if path, err := exec.LookPath("conmon"); err == nil {
		if err := probeConmon(path); err != nil {
			logrus.Warnf("Conmon at %s is invalid: %v", path, err)
			foundOutdatedConmon = true
		} else {
			logrus.Debugf("Using conmon from $PATH: %q", path)
			return path, nil
		}
	}

	if foundOutdatedConmon {
		return "", errors.Wrapf(define.ErrConmonOutdated,
			"please update to v%d.%d.%d or later",
			conmonMinMajorVersion, conmonMinMinorVersion, conmonMinPatchVersion)
	}

	return "", errors.Wrapf(define.ErrInvalidArg,
		"could not find a working conmon binary (configured options: %v)",
		conmonPaths)
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
		return errors.Wrap(err, define.ErrConmonVersionFormat)
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return errors.Wrap(err, define.ErrConmonVersionFormat)
	}
	if major < conmonMinMajorVersion {
		return define.ErrConmonOutdated
	}
	if major > conmonMinMajorVersion {
		return nil
	}

	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return errors.Wrap(err, define.ErrConmonVersionFormat)
	}
	if minor < conmonMinMinorVersion {
		return define.ErrConmonOutdated
	}
	if minor > conmonMinMinorVersion {
		return nil
	}

	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return errors.Wrap(err, define.ErrConmonVersionFormat)
	}
	if patch < conmonMinPatchVersion {
		return define.ErrConmonOutdated
	}
	if patch > conmonMinPatchVersion {
		return nil
	}

	return nil
}

// TmpDir gets the current Libpod temporary files directory.
func (r *Runtime) TmpDir() (string, error) {
	if !r.valid {
		return "", define.ErrRuntimeStopped
	}

	return r.config.Engine.TmpDir, nil
}

// GetConfig returns the configuration used by the runtime.
// Note that the returned value is not a copy and must hence
// only be used in a reading fashion.
func (r *Runtime) GetConfigNoCopy() (*config.Config, error) {
	if !r.valid {
		return nil, define.ErrRuntimeStopped
	}
	return r.config, nil
}

// GetConfig returns a copy of the configuration used by the runtime.
// Please use GetConfigNoCopy() in case you only want to read from
// but not write to the returned config.
func (r *Runtime) GetConfig() (*config.Config, error) {
	rtConfig, err := r.GetConfigNoCopy()
	if err != nil {
		return nil, err
	}

	config := new(config.Config)

	// Copy so the caller won't be able to modify the actual config
	if err := JSONDeepCopy(rtConfig, config); err != nil {
		return nil, errors.Wrapf(err, "error copying config")
	}

	return config, nil
}

// libimageEventsMap translates a libimage event type to a libpod event status.
var libimageEventsMap = map[libimage.EventType]events.Status{
	libimage.EventTypeImagePull:    events.Pull,
	libimage.EventTypeImagePush:    events.Push,
	libimage.EventTypeImageRemove:  events.Remove,
	libimage.EventTypeImageLoad:    events.LoadFromArchive,
	libimage.EventTypeImageSave:    events.Save,
	libimage.EventTypeImageTag:     events.Tag,
	libimage.EventTypeImageUntag:   events.Untag,
	libimage.EventTypeImageMount:   events.Mount,
	libimage.EventTypeImageUnmount: events.Unmount,
}

// libimageEvents spawns a goroutine in the background which is listenting for
// events on the libimage.Runtime.  The gourtine will be cleaned up implicitly
// when the main() exists.
func (r *Runtime) libimageEvents() {
	r.libimageEventsShutdown = make(chan bool)

	toLibpodEventStatus := func(e *libimage.Event) events.Status {
		status, found := libimageEventsMap[e.Type]
		if !found {
			return "Unknown"
		}
		return status
	}

	eventChannel := r.libimageRuntime.EventChannel()
	go func() {
		for {
			// Make sure to read and write all events before
			// checking if we're about to shutdown.
			for len(eventChannel) > 0 {
				libimageEvent := <-eventChannel
				e := events.Event{
					ID:     libimageEvent.ID,
					Name:   libimageEvent.Name,
					Status: toLibpodEventStatus(libimageEvent),
					Time:   libimageEvent.Time,
					Type:   events.Image,
				}
				if err := r.eventer.Write(e); err != nil {
					logrus.Errorf("Unable to write image event: %q", err)
				}
			}

			select {
			case <-r.libimageEventsShutdown:
				return

			default:
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}

// DeferredShutdown shuts down the runtime without exposing any
// errors. This is only meant to be used when the runtime is being
// shutdown within a defer statement; else use Shutdown
func (r *Runtime) DeferredShutdown(force bool) {
	_ = r.Shutdown(force)
}

// Shutdown shuts down the runtime and associated containers and storage
// If force is true, containers and mounted storage will be shut down before
// cleaning up; if force is false, an error will be returned if there are
// still containers running or mounted
func (r *Runtime) Shutdown(force bool) error {
	if !r.valid {
		return define.ErrRuntimeStopped
	}

	r.valid = false

	// Shutdown all containers if --force is given
	if force {
		ctrs, err := r.state.AllContainers()
		if err != nil {
			logrus.Errorf("Retrieving containers from database: %v", err)
		} else {
			for _, ctr := range ctrs {
				if err := ctr.StopWithTimeout(r.config.Engine.StopTimeout); err != nil {
					logrus.Errorf("Stopping container %s: %v", ctr.ID(), err)
				}
			}
		}
	}

	var lastError error
	// If no store was requested, it can be nil and there is no need to
	// attempt to shut it down
	if r.store != nil {
		// Wait for the events to be written.
		if r.libimageEventsShutdown != nil {
			r.libimageEventsShutdown <- true
		}

		// Note that the libimage runtime shuts down the store.
		if err := r.libimageRuntime.Shutdown(force); err != nil {
			lastError = errors.Wrapf(err, "error shutting down container storage")
		}
	}
	if err := r.state.Close(); err != nil {
		if lastError != nil {
			logrus.Error(lastError)
		}
		lastError = err
	}

	return lastError
}

// Reconfigures the runtime after a reboot
// Refreshes the state, recreating temporary files
// Does not check validity as the runtime is not valid until after this has run
func (r *Runtime) refresh(alivePath string) error {
	logrus.Debugf("Podman detected system restart - performing state refresh")

	// Clear state of database if not running in container
	if !graphRootMounted() {
		// First clear the state in the database
		if err := r.state.Refresh(); err != nil {
			return err
		}
	}

	// Next refresh the state of all containers to recreate dirs and
	// namespaces, and all the pods to recreate cgroups.
	// Containers, pods, and volumes must also reacquire their locks.
	ctrs, err := r.state.AllContainers()
	if err != nil {
		return errors.Wrapf(err, "error retrieving all containers from state")
	}
	pods, err := r.state.AllPods()
	if err != nil {
		return errors.Wrapf(err, "error retrieving all pods from state")
	}
	vols, err := r.state.AllVolumes()
	if err != nil {
		return errors.Wrapf(err, "error retrieving all volumes from state")
	}
	// No locks are taken during pod, volume, and container refresh.
	// Furthermore, the pod/volume/container refresh() functions are not
	// allowed to take locks themselves.
	// We cannot assume that any pod/volume/container has a valid lock until
	// after this function has returned.
	// The runtime alive lock should suffice to provide mutual exclusion
	// until this has run.
	for _, ctr := range ctrs {
		if err := ctr.refresh(); err != nil {
			logrus.Errorf("Refreshing container %s: %v", ctr.ID(), err)
		}
	}
	for _, pod := range pods {
		if err := pod.refresh(); err != nil {
			logrus.Errorf("Refreshing pod %s: %v", pod.ID(), err)
		}
	}
	for _, vol := range vols {
		if err := vol.refresh(); err != nil {
			logrus.Errorf("Refreshing volume %s: %v", vol.Name(), err)
		}
	}

	// Create a file indicating the runtime is alive and ready
	file, err := os.OpenFile(alivePath, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return errors.Wrap(err, "error creating runtime status file")
	}
	defer file.Close()

	r.newSystemEvent(events.Refresh)

	return nil
}

// Info returns the store and host information
func (r *Runtime) Info() (*define.Info, error) {
	return r.info()
}

// generateName generates a unique name for a container or pod.
func (r *Runtime) generateName() (string, error) {
	for {
		name := namesgenerator.GetRandomName(0)
		// Make sure container with this name does not exist
		if _, err := r.state.LookupContainer(name); err == nil {
			continue
		} else if errors.Cause(err) != define.ErrNoSuchCtr {
			return "", err
		}
		// Make sure pod with this name does not exist
		if _, err := r.state.LookupPod(name); err == nil {
			continue
		} else if errors.Cause(err) != define.ErrNoSuchPod {
			return "", err
		}
		return name, nil
	}
	// The code should never reach here.
}

// Configure store and image runtime
func (r *Runtime) configureStore() error {
	store, err := storage.GetStore(r.storageConfig)
	if err != nil {
		return err
	}

	r.store = store
	is.Transport.SetStore(store)

	// Set up a storage service for creating container root filesystems from
	// images
	r.storageService = getStorageService(r.store)

	runtimeOptions := &libimage.RuntimeOptions{
		SystemContext: r.imageContext,
	}
	libimageRuntime, err := libimage.RuntimeFromStore(store, runtimeOptions)
	if err != nil {
		return err
	}
	r.libimageRuntime = libimageRuntime
	// Run the libimage events routine.
	r.libimageEvents()

	return nil
}

// LibimageRuntime ... to allow for a step-by-step migration to libimage.
func (r *Runtime) LibimageRuntime() *libimage.Runtime {
	return r.libimageRuntime
}

// SystemContext returns the imagecontext
func (r *Runtime) SystemContext() *types.SystemContext {
	// Return the context from the libimage runtime.  libimage is sensitive
	// to a number of env vars.
	return r.libimageRuntime.SystemContext()
}

// GetOCIRuntimePath retrieves the path of the default OCI runtime.
func (r *Runtime) GetOCIRuntimePath() string {
	return r.defaultOCIRuntime.Path()
}

// DefaultOCIRuntime return copy of Default OCI Runtime
func (r *Runtime) DefaultOCIRuntime() OCIRuntime {
	return r.defaultOCIRuntime
}

// StorageConfig retrieves the storage options for the container runtime
func (r *Runtime) StorageConfig() storage.StoreOptions {
	return r.storageConfig
}

// RunRoot retrieves the current c/storage temporary directory in use by Libpod.
func (r *Runtime) RunRoot() string {
	if r.store == nil {
		return ""
	}
	return r.store.RunRoot()
}

// GetName retrieves the name associated with a given full ID.
// This works for both containers and pods, and does not distinguish between the
// two.
// If the given ID does not correspond to any existing Pod or Container,
// ErrNoSuchCtr is returned.
func (r *Runtime) GetName(id string) (string, error) {
	if !r.valid {
		return "", define.ErrRuntimeStopped
	}

	return r.state.GetName(id)
}

// DBConfig is a set of Libpod runtime configuration settings that are saved in
// a State when it is first created, and can subsequently be retrieved.
type DBConfig struct {
	LibpodRoot  string
	LibpodTmp   string
	StorageRoot string
	StorageTmp  string
	GraphDriver string
	VolumePath  string
}

// mergeDBConfig merges the configuration from the database.
func (r *Runtime) mergeDBConfig(dbConfig *DBConfig) {
	c := &r.config.Engine
	if !r.storageSet.RunRootSet && dbConfig.StorageTmp != "" {
		if r.storageConfig.RunRoot != dbConfig.StorageTmp &&
			r.storageConfig.RunRoot != "" {
			logrus.Debugf("Overriding run root %q with %q from database",
				r.storageConfig.RunRoot, dbConfig.StorageTmp)
		}
		r.storageConfig.RunRoot = dbConfig.StorageTmp
	}

	if !r.storageSet.GraphRootSet && dbConfig.StorageRoot != "" {
		if r.storageConfig.GraphRoot != dbConfig.StorageRoot &&
			r.storageConfig.GraphRoot != "" {
			logrus.Debugf("Overriding graph root %q with %q from database",
				r.storageConfig.GraphRoot, dbConfig.StorageRoot)
		}
		r.storageConfig.GraphRoot = dbConfig.StorageRoot
	}

	if !r.storageSet.GraphDriverNameSet && dbConfig.GraphDriver != "" {
		if r.storageConfig.GraphDriverName != dbConfig.GraphDriver &&
			r.storageConfig.GraphDriverName != "" {
			logrus.Errorf("User-selected graph driver %q overwritten by graph driver %q from database - delete libpod local files to resolve",
				r.storageConfig.GraphDriverName, dbConfig.GraphDriver)
		}
		r.storageConfig.GraphDriverName = dbConfig.GraphDriver
	}

	if !r.storageSet.StaticDirSet && dbConfig.LibpodRoot != "" {
		if c.StaticDir != dbConfig.LibpodRoot && c.StaticDir != "" {
			logrus.Debugf("Overriding static dir %q with %q from database", c.StaticDir, dbConfig.LibpodRoot)
		}
		c.StaticDir = dbConfig.LibpodRoot
	}

	if !r.storageSet.TmpDirSet && dbConfig.LibpodTmp != "" {
		if c.TmpDir != dbConfig.LibpodTmp && c.TmpDir != "" {
			logrus.Debugf("Overriding tmp dir %q with %q from database", c.TmpDir, dbConfig.LibpodTmp)
		}
		c.TmpDir = dbConfig.LibpodTmp
		if c.EventsLogFilePath == "" {
			c.EventsLogFilePath = filepath.Join(dbConfig.LibpodTmp, "events", "events.log")
		}
	}

	if !r.storageSet.VolumePathSet && dbConfig.VolumePath != "" {
		if c.VolumePath != dbConfig.VolumePath && c.VolumePath != "" {
			logrus.Debugf("Overriding volume path %q with %q from database", c.VolumePath, dbConfig.VolumePath)
		}
		c.VolumePath = dbConfig.VolumePath
	}
}

func (r *Runtime) EnableLabeling() bool {
	return r.config.Containers.EnableLabeling
}

// Reload reloads the configurations files
func (r *Runtime) Reload() error {
	if err := r.reloadContainersConf(); err != nil {
		return err
	}
	if err := r.reloadStorageConf(); err != nil {
		return err
	}
	// Invalidate the registries.conf cache. The next invocation will
	// reload all data.
	sysregistriesv2.InvalidateCache()
	return nil
}

// reloadContainersConf reloads the containers.conf
func (r *Runtime) reloadContainersConf() error {
	config, err := config.Reload()
	if err != nil {
		return err
	}
	r.config = config
	logrus.Infof("Applied new containers configuration: %v", config)
	return nil
}

// reloadStorageConf reloads the storage.conf
func (r *Runtime) reloadStorageConf() error {
	configFile, err := storage.DefaultConfigFile(rootless.IsRootless())
	if err != nil {
		return err
	}
	storage.ReloadConfigurationFile(configFile, &r.storageConfig)
	logrus.Infof("Applied new storage configuration: %v", r.storageConfig)
	return nil
}

// getVolumePlugin gets a specific volume plugin given its name.
func (r *Runtime) getVolumePlugin(name string) (*plugin.VolumePlugin, error) {
	// There is no plugin for local.
	if name == define.VolumeDriverLocal || name == "" {
		return nil, nil
	}

	pluginPath, ok := r.config.Engine.VolumePlugins[name]
	if !ok {
		return nil, errors.Wrapf(define.ErrMissingPlugin, "no volume plugin with name %s available", name)
	}

	return plugin.GetVolumePlugin(name, pluginPath)
}

// GetSecretsStoreageDir returns the directory that the secrets manager should take
func (r *Runtime) GetSecretsStorageDir() string {
	return filepath.Join(r.store.GraphRoot(), "secrets")
}

// SecretsManager returns the directory that the secrets manager should take
func (r *Runtime) SecretsManager() (*secrets.SecretsManager, error) {
	if r.secretsManager == nil {
		manager, err := secrets.NewManager(r.GetSecretsStorageDir())
		if err != nil {
			return nil, err
		}
		r.secretsManager = manager
	}
	return r.secretsManager, nil
}

func graphRootMounted() bool {
	f, err := os.OpenFile("/run/.containerenv", os.O_RDONLY, os.ModePerm)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if scanner.Text() == "graphRootMounted=1" {
			return true
		}
	}
	return false
}

func (r *Runtime) graphRootMountedFlag(mounts []spec.Mount) string {
	root := r.store.GraphRoot()
	for _, val := range mounts {
		if strings.HasPrefix(root, val.Source) {
			return "graphRootMounted=1"
		}
	}
	return ""
}

// Network returns the network interface which is used by the runtime
func (r *Runtime) Network() nettypes.ContainerNetwork {
	return r.network
}

// Network returns the network interface which is used by the runtime
func (r *Runtime) GetDefaultNetworkName() string {
	return r.config.Network.DefaultNetwork
}
