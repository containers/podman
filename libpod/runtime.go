package libpod

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/containers/common/pkg/config"
	is "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/libpod/events"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/libpod/lock"
	"github.com/containers/libpod/pkg/cgroups"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/storage"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/pkg/errors"
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

	state             State
	store             storage.Store
	storageService    *storageService
	imageContext      *types.SystemContext
	defaultOCIRuntime OCIRuntime
	ociRuntimes       map[string]OCIRuntime
	netPlugin         ocicni.CNIPlugin
	conmonPath        string
	imageRuntime      *image.Runtime
	lockManager       lock.Manager

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
	lock  sync.RWMutex

	// mechanism to read and write even logs
	eventer events.Eventer

	// noStore indicates whether we need to interact with a store or not
	noStore bool
}

// SetXdgDirs ensures the XDG_RUNTIME_DIR env and XDG_CONFIG_HOME variables are set.
// containers/image uses XDG_RUNTIME_DIR to locate the auth file, XDG_CONFIG_HOME is
// use for the libpod.conf configuration file.
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
func NewRuntime(ctx context.Context, options ...RuntimeOption) (runtime *Runtime, err error) {
	conf, err := config.NewConfig("")
	if err != nil {
		return nil, err
	}
	conf.CheckCgroupsAndAdjustConfig()
	return newRuntimeFromConfig(ctx, conf, options...)
}

// NewRuntimeFromConfig creates a new container runtime using the given
// configuration file for its default configuration. Passed RuntimeOption
// functions can be used to mutate this configuration further.
// An error will be returned if the configuration file at the given path does
// not exist or cannot be loaded
func NewRuntimeFromConfig(ctx context.Context, userConfig *config.Config, options ...RuntimeOption) (runtime *Runtime, err error) {

	return newRuntimeFromConfig(ctx, userConfig, options...)
}

func newRuntimeFromConfig(ctx context.Context, conf *config.Config, options ...RuntimeOption) (runtime *Runtime, err error) {
	runtime = new(Runtime)

	if conf.Engine.OCIRuntime == "" {
		conf.Engine.OCIRuntime = "runc"
		// If we're running on cgroups v2, default to using crun.
		if onCgroupsv2, _ := cgroups.IsCgroup2UnifiedMode(); onCgroupsv2 {
			conf.Engine.OCIRuntime = "crun"
		}
	}

	runtime.config = conf

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

	if err := makeRuntime(ctx, runtime); err != nil {
		return nil, err
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
func makeRuntime(ctx context.Context, runtime *Runtime) (err error) {
	// Find a working conmon binary
	cPath, err := runtime.config.FindConmon()
	if err != nil {
		return err
	}
	runtime.conmonPath = cPath

	// Make the static files directory if it does not exist
	if err := os.MkdirAll(runtime.config.Engine.StaticDir, 0700); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return errors.Wrapf(err, "error creating runtime static files directory %s",
				runtime.config.Engine.StaticDir)
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
		state, err := NewInMemoryState()
		if err != nil {
			return err
		}
		runtime.state = state
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

	if err := runtime.mergeDBConfig(dbConfig); err != nil {
		return errors.Wrapf(err, "error merging database config into runtime config")
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

	// Set up containers/storage
	var store storage.Store
	if os.Geteuid() != 0 {
		logrus.Debug("Not configuring container store")
	} else if runtime.noStore {
		logrus.Debug("No store required. Not opening container store.")
	} else if err := runtime.configureStore(); err != nil {
		return err
	}
	defer func() {
		if err != nil && store != nil {
			// Don't forcibly shut down
			// We could be opening a store in use by another libpod
			_, err2 := store.Shutdown(false)
			if err2 != nil {
				logrus.Errorf("Error removing store for partially-created runtime: %s", err2)
			}
		}
	}()

	// Setup the eventer
	eventer, err := runtime.newEventer()
	if err != nil {
		return err
	}
	runtime.eventer = eventer
	if runtime.imageRuntime != nil {
		runtime.imageRuntime.Eventer = eventer
	}

	// Set up containers/image
	if runtime.imageContext == nil {
		runtime.imageContext = &types.SystemContext{}
	}
	runtime.imageContext.SignaturePolicyPath = runtime.config.Engine.SignaturePolicyPath

	// Create the tmpDir
	if err := os.MkdirAll(runtime.config.Engine.TmpDir, 0751); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return errors.Wrapf(err, "error creating tmpdir %s", runtime.config.Engine.TmpDir)
		}
	}

	// Create events log dir
	if err := os.MkdirAll(filepath.Dir(runtime.config.Engine.EventsLogFilePath), 0700); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return errors.Wrapf(err, "error creating events dirs %s", filepath.Dir(runtime.config.Engine.EventsLogFilePath))
		}
	}

	// Get us at least one working OCI runtime.
	runtime.ociRuntimes = make(map[string]OCIRuntime)

	// Initialize remaining OCI runtimes
	for name, paths := range runtime.config.Engine.OCIRuntimes {

		ociRuntime, err := newConmonOCIRuntime(name, paths, runtime.conmonPath, runtime.config)
		if err != nil {
			// Don't fatally error.
			// This will allow us to ship configs including optional
			// runtimes that might not be installed (crun, kata).
			// Only a warnf so default configs don't spec errors.
			logrus.Warnf("Error initializing configured OCI runtime %s: %v", name, err)
			continue
		}

		runtime.ociRuntimes[name] = ociRuntime
	}

	// Do we have a default OCI runtime?
	if runtime.config.Engine.OCIRuntime != "" {
		// If the string starts with / it's a path to a runtime
		// executable.
		if strings.HasPrefix(runtime.config.Engine.OCIRuntime, "/") {
			name := filepath.Base(runtime.config.Engine.OCIRuntime)

			ociRuntime, err := newConmonOCIRuntime(name, []string{runtime.config.Engine.OCIRuntime}, runtime.conmonPath, runtime.config)
			if err != nil {
				return err
			}

			runtime.ociRuntimes[name] = ociRuntime
			runtime.defaultOCIRuntime = ociRuntime
		} else {
			ociRuntime, ok := runtime.ociRuntimes[runtime.config.Engine.OCIRuntime]
			if !ok {
				return errors.Wrapf(define.ErrInvalidArg, "default OCI runtime %q not found", runtime.config.Engine.OCIRuntime)
			}
			runtime.defaultOCIRuntime = ociRuntime
		}
	}

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
			return errors.Wrapf(err, "error creating runtime temporary files directory %s",
				runtime.config.Engine.TmpDir)
		}
	}

	// Set up the CNI net plugin
	if !rootless.IsRootless() {

		netPlugin, err := ocicni.InitCNI(runtime.config.Network.DefaultNetwork, runtime.config.Network.NetworkConfigDir, runtime.config.Network.CNIPluginDirs...)
		if err != nil {
			return errors.Wrapf(err, "error configuring CNI network plugin")
		}
		runtime.netPlugin = netPlugin
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
		if os.Geteuid() != 0 {
			aliveLock.Unlock() // Unlock to avoid deadlock as BecomeRootInUserNS will reexec.
			pausePid, err := util.GetRootlessPauseProcessPidPath()
			if err != nil {
				return errors.Wrapf(err, "could not get pause process pid file path")
			}
			became, ret, err := rootless.BecomeRootInUserNS(pausePid)
			if err != nil {
				return err
			}
			if became {
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

// GetConfig returns a copy of the configuration used by the runtime
func (r *Runtime) GetConfig() (*config.Config, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, define.ErrRuntimeStopped
	}

	config := new(config.Config)

	// Copy so the caller won't be able to modify the actual config
	if err := JSONDeepCopy(r.config, config); err != nil {
		return nil, errors.Wrapf(err, "error copying config")
	}

	return config, nil
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
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.valid {
		return define.ErrRuntimeStopped
	}

	r.valid = false

	// Shutdown all containers if --force is given
	if force {
		ctrs, err := r.state.AllContainers()
		if err != nil {
			logrus.Errorf("Error retrieving containers from database: %v", err)
		} else {
			for _, ctr := range ctrs {
				if err := ctr.StopWithTimeout(r.config.Engine.StopTimeout); err != nil {
					logrus.Errorf("Error stopping container %s: %v", ctr.ID(), err)
				}
			}
		}
	}

	var lastError error
	// If no store was requested, it can bew nil and there is no need to
	// attempt to shut it down
	if r.store != nil {
		if _, err := r.store.Shutdown(force); err != nil {
			lastError = errors.Wrapf(err, "Error shutting down container storage")
		}
	}
	if err := r.state.Close(); err != nil {
		if lastError != nil {
			logrus.Errorf("%v", lastError)
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

	// First clear the state in the database
	if err := r.state.Refresh(); err != nil {
		return err
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
			logrus.Errorf("Error refreshing container %s: %v", ctr.ID(), err)
		}
	}
	for _, pod := range pods {
		if err := pod.refresh(); err != nil {
			logrus.Errorf("Error refreshing pod %s: %v", pod.ID(), err)
		}
	}
	for _, vol := range vols {
		if err := vol.refresh(); err != nil {
			logrus.Errorf("Error refreshing volume %s: %v", vol.Name(), err)
		}
	}

	// Create a file indicating the runtime is alive and ready
	file, err := os.OpenFile(alivePath, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return errors.Wrapf(err, "error creating runtime status file %s", alivePath)
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
	storageService, err := getStorageService(r.store)
	if err != nil {
		return err
	}
	r.storageService = storageService

	ir := image.NewImageRuntimeFromStore(r.store)
	ir.SignaturePolicyPath = r.config.Engine.SignaturePolicyPath
	ir.EventsLogFilePath = r.config.Engine.EventsLogFilePath
	ir.EventsLogger = r.config.Engine.EventsLogger

	r.imageRuntime = ir

	return nil
}

// ImageRuntime returns the imageruntime for image operations.
// If WithNoStore() was used, no image runtime will be available, and this
// function will return nil.
func (r *Runtime) ImageRuntime() *image.Runtime {
	return r.imageRuntime
}

// SystemContext returns the imagecontext
func (r *Runtime) SystemContext() *types.SystemContext {
	return r.imageContext
}

// GetOCIRuntimePath retrieves the path of the default OCI runtime.
func (r *Runtime) GetOCIRuntimePath() string {
	return r.defaultOCIRuntime.Path()
}

// StorageConfig retrieves the storage options for the container runtime
func (r *Runtime) StorageConfig() storage.StoreOptions {
	return r.storageConfig
}

// GetStore returns the runtime stores
func (r *Runtime) GetStore() storage.Store {
	return r.store
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
func (r *Runtime) mergeDBConfig(dbConfig *DBConfig) error {

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
		c.EventsLogFilePath = filepath.Join(dbConfig.LibpodTmp, "events", "events.log")
	}

	if !r.storageSet.VolumePathSet && dbConfig.VolumePath != "" {
		if c.VolumePath != dbConfig.VolumePath && c.VolumePath != "" {
			logrus.Debugf("Overriding volume path %q with %q from database", c.VolumePath, dbConfig.VolumePath)
		}
		c.VolumePath = dbConfig.VolumePath
	}
	return nil
}

func (r *Runtime) EnableLabeling() bool {
	return r.config.Containers.EnableLabeling
}
