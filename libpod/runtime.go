package libpod

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	"github.com/containers/storage/pkg/lockfile"
	"github.com/containers/storage/pkg/unshare"
	"github.com/docker/docker/pkg/namesgenerator"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
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

	// Worker
	workerChannel chan func()
	workerGroup   sync.WaitGroup

	// syslog describes whenever logrus should log to the syslog as well.
	// Note that the syslog hook will be enabled early in cmd/podman/syslog_linux.go
	// This bool is just needed so that we can set it for netavark interface.
	syslog bool

	// doReset indicates that the runtime should perform a system reset.
	// All Podman files will be removed.
	doReset bool

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

func init() {
	// generateName calls namesgenerator.GetRandomName which the
	// global RNG from math/rand. Seed it here to make sure we
	// don't get the same name every time.
	rand.Seed(time.Now().UnixNano())
}

// SetXdgDirs ensures the XDG_RUNTIME_DIR env and XDG_CONFIG_HOME variables are set.
// containers/image uses XDG_RUNTIME_DIR to locate the auth file, XDG_CONFIG_HOME is
// use for the containers.conf configuration file.
func SetXdgDirs() error {
	if !rootless.IsRootless() {
		return nil
	}

	// Set up XDG_RUNTIME_DIR
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")

	if runtimeDir == "" {
		var err error
		runtimeDir, err = util.GetRuntimeDir()
		if err != nil {
			return err
		}
	}
	if err := os.Setenv("XDG_RUNTIME_DIR", runtimeDir); err != nil {
		return fmt.Errorf("cannot set XDG_RUNTIME_DIR: %w", err)
	}

	if rootless.IsRootless() && os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		sessionAddr := filepath.Join(runtimeDir, "bus")
		if _, err := os.Stat(sessionAddr); err == nil {
			os.Setenv("DBUS_SESSION_BUS_ADDRESS", fmt.Sprintf("unix:path=%s", sessionAddr))
		}
	}

	// Set up XDG_CONFIG_HOME
	if cfgHomeDir := os.Getenv("XDG_CONFIG_HOME"); cfgHomeDir == "" {
		cfgHomeDir, err := util.GetRootlessConfigHomeDir()
		if err != nil {
			return err
		}
		if err := os.Setenv("XDG_CONFIG_HOME", cfgHomeDir); err != nil {
			return fmt.Errorf("cannot set XDG_CONFIG_HOME: %w", err)
		}
	}
	return nil
}

// NewRuntime creates a new container runtime
// Options can be passed to override the default configuration for the runtime
func NewRuntime(ctx context.Context, options ...RuntimeOption) (*Runtime, error) {
	conf, err := config.Default()
	if err != nil {
		return nil, err
	}
	return newRuntimeFromConfig(conf, options...)
}

// NewRuntimeFromConfig creates a new container runtime using the given
// configuration file for its default configuration. Passed RuntimeOption
// functions can be used to mutate this configuration further.
// An error will be returned if the configuration file at the given path does
// not exist or cannot be loaded
func NewRuntimeFromConfig(ctx context.Context, userConfig *config.Config, options ...RuntimeOption) (*Runtime, error) {
	return newRuntimeFromConfig(userConfig, options...)
}

func newRuntimeFromConfig(conf *config.Config, options ...RuntimeOption) (*Runtime, error) {
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
			return nil, fmt.Errorf("configuring runtime: %w", err)
		}
	}

	if err := shutdown.Register("libpod", func(sig os.Signal) error {
		// For `systemctl stop podman.service` support, exit code should be 0
		if sig == syscall.SIGTERM {
			os.Exit(0)
		}
		os.Exit(1)
		return nil
	}); err != nil && !errors.Is(err, shutdown.ErrHandlerExists) {
		logrus.Errorf("Registering shutdown handler for libpod: %v", err)
	}

	if err := shutdown.Start(); err != nil {
		return nil, fmt.Errorf("starting shutdown signal handler: %w", err)
	}

	if err := makeRuntime(runtime); err != nil {
		return nil, err
	}

	runtime.config.CheckCgroupsAndAdjustConfig()

	// If resetting storage, do *not* return a runtime.
	if runtime.doReset {
		return nil, nil
	}

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
			if errors.Is(err, os.ErrNotExist) {
				manager, err = lock.NewFileLockManager(lockPath)
				if err != nil {
					return nil, fmt.Errorf("failed to get new file lock manager: %w", err)
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
			case errors.Is(err, os.ErrNotExist):
				manager, err = lock.NewSHMLockManager(lockPath, runtime.config.Engine.NumLocks)
				if err != nil {
					return nil, fmt.Errorf("failed to get new shm lock manager: %w", err)
				}
			case errors.Is(err, syscall.ERANGE) && runtime.doRenumber:
				logrus.Debugf("Number of locks does not match - removing old locks")

				// ERANGE indicates a lock numbering mismatch.
				// Since we're renumbering, this is not fatal.
				// Remove the earlier set of locks and recreate.
				if err := os.Remove(filepath.Join("/dev/shm", lockPath)); err != nil {
					return nil, fmt.Errorf("removing libpod locks file %s: %w", lockPath, err)
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
		return nil, fmt.Errorf("unknown lock type %s: %w", runtime.config.Engine.LockType, define.ErrInvalidArg)
	}
	return manager, nil
}

// Make a new runtime based on the given configuration
// Sets up containers/storage, state store, OCI runtime
func makeRuntime(runtime *Runtime) (retErr error) {
	// Find a working conmon binary
	cPath, err := runtime.config.FindConmon()
	if err != nil {
		return err
	}
	runtime.conmonPath = cPath

	if runtime.noStore && runtime.doReset {
		return fmt.Errorf("cannot perform system reset if runtime is not creating a store: %w", define.ErrInvalidArg)
	}
	if runtime.doReset && runtime.doRenumber {
		return fmt.Errorf("cannot perform system reset while renumbering locks: %w", define.ErrInvalidArg)
	}

	// Make the static files directory if it does not exist
	if err := os.MkdirAll(runtime.config.Engine.StaticDir, 0700); err != nil {
		// The directory is allowed to exist
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("creating runtime static files directory: %w", err)
		}
	}

	// Create the TmpDir if needed
	if err := os.MkdirAll(runtime.config.Engine.TmpDir, 0751); err != nil {
		return fmt.Errorf("creating runtime temporary files directory: %w", err)
	}

	// Set up the state.
	//
	// TODO - if we further break out the state implementation into
	// libpod/state, the config could take care of the code below.  It
	// would further allow to move the types and consts into a coherent
	// package.
	switch runtime.config.Engine.StateType {
	case config.InMemoryStateStore:
		return fmt.Errorf("in-memory state is currently disabled: %w", define.ErrInvalidArg)
	case config.SQLiteStateStore:
		return fmt.Errorf("SQLite state is currently disabled: %w", define.ErrInvalidArg)
	case config.BoltDBStateStore:
		baseDir := runtime.config.Engine.StaticDir
		if runtime.storageConfig.TransientStore {
			baseDir = runtime.config.Engine.TmpDir
		}
		dbPath := filepath.Join(baseDir, "bolt_state.db")

		state, err := NewBoltState(dbPath, runtime)
		if err != nil {
			return err
		}
		runtime.state = state
	default:
		return fmt.Errorf("unrecognized state type passed (%v): %w", runtime.config.Engine.StateType, define.ErrInvalidArg)
	}

	// Grab config from the database so we can reset some defaults
	dbConfig, err := runtime.state.GetDBConfig()
	if err != nil {
		if runtime.doReset {
			// We can at least delete the DB and the static files
			// directory.
			// Can't safely touch anything else because we aren't
			// sure of other directories.
			if err := runtime.state.Close(); err != nil {
				logrus.Errorf("Closing database connection: %v", err)
			} else {
				if err := os.RemoveAll(runtime.config.Engine.StaticDir); err != nil {
					logrus.Errorf("Removing static files directory %v: %v", runtime.config.Engine.StaticDir, err)
				}
			}
		}

		return fmt.Errorf("retrieving runtime configuration from database: %w", err)
	}

	runtime.mergeDBConfig(dbConfig)

	unified, _ := cgroups.IsCgroup2UnifiedMode()
	if unified && rootless.IsRootless() && !systemd.IsSystemdSessionValid(rootless.GetRootlessUID()) {
		// If user is rootless and XDG_RUNTIME_DIR is found, podman will not proceed with /tmp directory
		// it will try to use existing XDG_RUNTIME_DIR
		// if current user has no write access to XDG_RUNTIME_DIR we will fail later
		if err := unix.Access(runtime.storageConfig.RunRoot, unix.W_OK); err != nil {
			msg := fmt.Sprintf("RunRoot is pointing to a path (%s) which is not writable. Most likely podman will fail.", runtime.storageConfig.RunRoot)
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
	logrus.Debugf("Using transient store: %v", runtime.storageConfig.TransientStore)

	// Validate our config against the database, now that we've set our
	// final storage configuration
	if err := runtime.state.ValidateDBConfig(runtime); err != nil {
		// If we are performing a storage reset: continue on with a
		// warning. Otherwise we can't `system reset` after a change to
		// the core paths.
		if !runtime.doReset {
			return err
		}
		logrus.Errorf("Runtime paths differ from those stored in database, storage reset may not remove all files")
	}

	if err := runtime.state.SetNamespace(runtime.config.Engine.Namespace); err != nil {
		return fmt.Errorf("setting libpod namespace in state: %w", err)
	}
	logrus.Debugf("Set libpod namespace to %q", runtime.config.Engine.Namespace)

	needsUserns := os.Geteuid() != 0
	if !needsUserns {
		hasCapSysAdmin, err := unshare.HasCapSysAdmin()
		if err != nil {
			return err
		}
		needsUserns = !hasCapSysAdmin
	}
	// Set up containers/storage
	var store storage.Store
	if needsUserns {
		logrus.Debug("Not configuring container store")
	} else if runtime.noStore {
		logrus.Debug("No store required. Not opening container store.")
	} else if err := runtime.configureStore(); err != nil {
		// Make a best-effort attempt to clean up if performing a
		// storage reset.
		if runtime.doReset {
			if err := runtime.removeAllDirs(); err != nil {
				logrus.Errorf("Removing libpod directories: %v", err)
			}
		}

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

	// Set up the eventer
	eventer, err := runtime.newEventer()
	if err != nil {
		return err
	}
	runtime.eventer = eventer

	// Set up containers/image
	if runtime.imageContext == nil {
		runtime.imageContext = &types.SystemContext{
			BigFilesTemporaryDir: parse.GetTempDir(),
		}
	}
	runtime.imageContext.SignaturePolicyPath = runtime.config.Engine.SignaturePolicyPath

	// Get us at least one working OCI runtime.
	runtime.ociRuntimes = make(map[string]OCIRuntime)

	// Initialize remaining OCI runtimes
	for name, paths := range runtime.config.Engine.OCIRuntimes {
		ociRuntime, err := newConmonOCIRuntime(name, paths, runtime.conmonPath, runtime.runtimeFlags, runtime.config)
		if err != nil {
			// Don't fatally error.
			// This will allow us to ship configs including optional
			// runtimes that might not be installed (crun, kata).
			// Only an infof so default configs don't spec errors.
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
				return fmt.Errorf("default OCI runtime %q not found: %w", runtime.config.Engine.OCIRuntime, define.ErrInvalidArg)
			}
			runtime.defaultOCIRuntime = ociRuntime
		}
	}
	logrus.Debugf("Using OCI runtime %q", runtime.defaultOCIRuntime.Path())

	// Do we have at least one valid OCI runtime?
	if len(runtime.ociRuntimes) == 0 {
		return fmt.Errorf("no OCI runtime has been configured: %w", define.ErrInvalidArg)
	}

	// Do we have a default runtime?
	if runtime.defaultOCIRuntime == nil {
		return fmt.Errorf("no default OCI runtime was configured: %w", define.ErrInvalidArg)
	}

	// the store is only set up when we are in the userns so we do the same for the network interface
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
	aliveLock, err := lockfile.GetLockFile(runtimeAliveLock)
	if err != nil {
		return fmt.Errorf("acquiring runtime init lock: %w", err)
	}
	// Acquire the lock and hold it until we return
	// This ensures that no two processes will be in runtime.refresh at once
	aliveLock.Lock()
	doRefresh := false
	unLockFunc := aliveLock.Unlock
	defer func() {
		if unLockFunc != nil {
			unLockFunc()
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
			unLockFunc()
			unLockFunc = nil
			pausePid, err := util.GetRootlessPauseProcessPidPathGivenDir(runtime.config.Engine.TmpDir)
			if err != nil {
				return fmt.Errorf("could not get pause process pid file path: %w", err)
			}
			became, ret, err := rootless.BecomeRootInUserNS(pausePid)
			if err != nil {
				return err
			}
			if became {
				// Check if the pause process was created.  If it was created, then
				// move it to its own systemd scope.
				utils.MovePauseProcessToScope(pausePid)

				// gocritic complains because defer is not run on os.Exit()
				// However this is fine because the lock is released anyway when the process exits
				//nolint:gocritic
				os.Exit(ret)
			}
		}
		// If the file doesn't exist, we need to refresh the state
		// This will trigger on first use as well, but refreshing an
		// empty state only creates a single file
		// As such, it's not really a performance concern
		if errors.Is(err, os.ErrNotExist) {
			doRefresh = true
		} else {
			return fmt.Errorf("reading runtime status file %s: %w", runtimeAliveFile, err)
		}
	}

	runtime.lockManager, err = getLockManager(runtime)
	if err != nil {
		return err
	}

	// If we're resetting storage, do it now.
	// We will not return a valid runtime.
	// TODO: Plumb this context out so it can be set.
	if runtime.doReset {
		// Mark the runtime as valid, so normal functionality "mostly"
		// works and we can use regular functions to remove
		// ctrs/pods/etc
		runtime.valid = true

		return runtime.reset(context.Background())
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

	runtime.startWorker()

	// Mark the runtime as valid - ready to be used, cannot be modified
	// further
	runtime.valid = true

	if runtime.doMigrate {
		if err := runtime.migrate(); err != nil {
			return err
		}
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
		return nil, fmt.Errorf("copying config: %w", err)
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
		sawShutdown := false
		for {
			// Make sure to read and write all events before
			// shutting down.
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

			if sawShutdown {
				close(r.libimageEventsShutdown)
				return
			}

			select {
			case <-r.libimageEventsShutdown:
				sawShutdown = true
			case <-time.After(100 * time.Millisecond):
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
		return nil
	}

	if r.workerChannel != nil {
		r.workerGroup.Wait()
		close(r.workerChannel)
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
			// Tell loop to shutdown
			r.libimageEventsShutdown <- true
			// Wait for close to signal shutdown
			<-r.libimageEventsShutdown
		}

		// Note that the libimage runtime shuts down the store.
		if err := r.libimageRuntime.Shutdown(force); err != nil {
			lastError = fmt.Errorf("shutting down container storage: %w", err)
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
		return fmt.Errorf("retrieving all containers from state: %w", err)
	}
	pods, err := r.state.AllPods()
	if err != nil {
		return fmt.Errorf("retrieving all pods from state: %w", err)
	}
	vols, err := r.state.AllVolumes()
	if err != nil {
		return fmt.Errorf("retrieving all volumes from state: %w", err)
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
		return fmt.Errorf("creating runtime status file: %w", err)
	}
	defer file.Close()

	r.NewSystemEvent(events.Refresh)

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
		} else if !errors.Is(err, define.ErrNoSuchCtr) {
			return "", err
		}
		// Make sure pod with this name does not exist
		if _, err := r.state.LookupPod(name); err == nil {
			continue
		} else if !errors.Is(err, define.ErrNoSuchPod) {
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

func (r *Runtime) GarbageCollect() error {
	return r.store.GarbageCollect()
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
			logrus.Errorf("User-selected graph driver %q overwritten by graph driver %q from database - delete libpod local files (%q) to resolve.  May prevent use of images created by other tools",
				r.storageConfig.GraphDriverName, dbConfig.GraphDriver, r.storageConfig.GraphRoot)
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

// getVolumePlugin gets a specific volume plugin.
func (r *Runtime) getVolumePlugin(volConfig *VolumeConfig) (*plugin.VolumePlugin, error) {
	// There is no plugin for local.
	name := volConfig.Driver
	timeout := volConfig.Timeout
	if name == define.VolumeDriverLocal || name == "" {
		return nil, nil
	}

	pluginPath, ok := r.config.Engine.VolumePlugins[name]
	if !ok {
		if name == define.VolumeDriverImage {
			return nil, nil
		}
		return nil, fmt.Errorf("no volume plugin with name %s available: %w", name, define.ErrMissingPlugin)
	}

	return plugin.GetVolumePlugin(name, pluginPath, timeout, r.config)
}

// GetSecretsStorageDir returns the directory that the secrets manager should take
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

// GetDefaultNetworkName returns the network interface which is used by the runtime
func (r *Runtime) GetDefaultNetworkName() string {
	return r.config.Network.DefaultNetwork
}

// RemoteURI returns the API server URI
func (r *Runtime) RemoteURI() string {
	return r.config.Engine.RemoteURI
}

// SetRemoteURI records the API server URI
func (r *Runtime) SetRemoteURI(uri string) {
	r.config.Engine.RemoteURI = uri
}
