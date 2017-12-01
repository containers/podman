package libpod

import (
	"os"
	"path/filepath"
	"sync"

	is "github.com/containers/image/storage"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/ulule/deepcopier"
)

// A RuntimeOption is a functional option which alters the Runtime created by
// NewRuntime
type RuntimeOption func(*Runtime) error

// Runtime is the core libpod runtime
type Runtime struct {
	config         *RuntimeConfig
	state          State
	store          storage.Store
	storageService *storageService
	imageContext   *types.SystemContext
	ociRuntime     *OCIRuntime
	valid          bool
	lock           sync.RWMutex
}

// RuntimeConfig contains configuration options used to set up the runtime
type RuntimeConfig struct {
	StorageConfig         storage.StoreOptions
	ImageDefaultTransport string
	InsecureRegistries    []string
	Registries            []string
	SignaturePolicyPath   string
	InMemoryState         bool
	RuntimePath           string
	ConmonPath            string
	ConmonEnvVars         []string
	CgroupManager         string
	StaticDir             string
	TmpDir                string
	SelinuxEnabled        bool
	PidsLimit             int64
	MaxLogSize            int64
	NoPivotRoot           bool
}

var (
	defaultRuntimeConfig = RuntimeConfig{
		// Leave this empty so containers/storage will use its defaults
		StorageConfig:         storage.StoreOptions{},
		ImageDefaultTransport: DefaultTransport,
		InMemoryState:         false,
		RuntimePath:           "/usr/bin/runc",
		ConmonPath:            "/usr/local/libexec/crio/conmon",
		ConmonEnvVars: []string{
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		},
		CgroupManager:  "cgroupfs",
		StaticDir:      filepath.Join(storage.DefaultStoreOptions.GraphRoot, "libpod"),
		TmpDir:         "/var/run/libpod",
		SelinuxEnabled: false,
		PidsLimit:      1024,
		MaxLogSize:     -1,
		NoPivotRoot:    false,
	}
)

// NewRuntime creates a new container runtime
// Options can be passed to override the default configuration for the runtime
func NewRuntime(options ...RuntimeOption) (runtime *Runtime, err error) {
	runtime = new(Runtime)
	runtime.config = new(RuntimeConfig)

	// Copy the default configuration
	deepcopier.Copy(defaultRuntimeConfig).To(runtime.config)

	// Overwrite it with user-given configuration options
	for _, opt := range options {
		if err := opt(runtime); err != nil {
			return nil, errors.Wrapf(err, "error configuring runtime")
		}
	}

	// Set up containers/storage
	store, err := storage.GetStore(runtime.config.StorageConfig)
	if err != nil {
		return nil, err
	}
	runtime.store = store
	is.Transport.SetStore(store)
	defer func() {
		if err != nil {
			// Don't forcibly shut down
			// We could be opening a store in use by another libpod
			_, err2 := store.Shutdown(false)
			if err2 != nil {
				logrus.Errorf("Error removing store for partially-created runtime: %s", err2)
			}
		}
	}()

	// Set up a storage service for creating container root filesystems from
	// images
	storageService, err := getStorageService(runtime.store)
	if err != nil {
		return nil, err
	}
	runtime.storageService = storageService

	// Set up containers/image
	runtime.imageContext = &types.SystemContext{
		SignaturePolicyPath: runtime.config.SignaturePolicyPath,
	}

	// Make an OCI runtime to perform container operations
	ociRuntime, err := newOCIRuntime("runc", runtime.config.RuntimePath,
		runtime.config.ConmonPath, runtime.config.ConmonEnvVars,
		runtime.config.CgroupManager, runtime.config.TmpDir,
		runtime.config.MaxLogSize, runtime.config.NoPivotRoot)
	if err != nil {
		return nil, err
	}
	runtime.ociRuntime = ociRuntime

	// Make the static files directory if it does not exist
	if err := os.MkdirAll(runtime.config.StaticDir, 0755); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return nil, errors.Wrapf(err, "error creating runtime static files directory %s",
				runtime.config.StaticDir)
		}
	}

	// Make the per-boot files directory if it does not exist
	if err := os.MkdirAll(runtime.config.TmpDir, 0755); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return nil, errors.Wrapf(err, "error creating runtime temporary files directory %s",
				runtime.config.TmpDir)
		}
	}

	// Set up the state
	if runtime.config.InMemoryState {
		state, err := NewInMemoryState()
		if err != nil {
			return nil, err
		}
		runtime.state = state
	} else {
		dbPath := filepath.Join(runtime.config.StaticDir, "state.sql")
		lockPath := filepath.Join(runtime.config.TmpDir, "state.lck")
		specsDir := filepath.Join(runtime.config.StaticDir, "ocispec")

		// Make a directory to hold JSON versions of container OCI specs
		if err := os.MkdirAll(specsDir, 0755); err != nil {
			// The directory is allowed to exist
			if !os.IsExist(err) {
				return nil, errors.Wrapf(err, "error creating runtime OCI specs directory %s",
					specsDir)
			}
		}

		state, err := NewSQLState(dbPath, lockPath, specsDir, runtime)
		if err != nil {
			return nil, err
		}
		runtime.state = state
	}

	// We now need to see if the system has restarted
	// We check for the presence of a file in our tmp directory to verify this
	runtimeAliveFile := filepath.Join(runtime.config.TmpDir, "alive")
	_, err = os.Stat(runtimeAliveFile)
	if err != nil {
		// If the file doesn't exist, we need to refresh the state
		// This will trigger on first use as well, but refreshing an
		// empty state only creates a single file
		// As such, it's not really a performance concern
		if os.IsNotExist(err) {
			if err2 := runtime.refresh(runtimeAliveFile); err2 != nil {
				return nil, err2
			}
		} else {
			return nil, errors.Wrapf(err, "error reading runtime status file %s", runtimeAliveFile)
		}
	}

	// Mark the runtime as valid - ready to be used, cannot be modified
	// further
	runtime.valid = true

	return runtime, nil
}

// GetConfig returns a copy of the configuration used by the runtime
func (r *Runtime) GetConfig() *RuntimeConfig {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil
	}

	config := new(RuntimeConfig)

	// Copy so the caller won't be able to modify the actual config
	deepcopier.Copy(r.config).To(config)

	return config
}

// Shutdown shuts down the runtime and associated containers and storage
// If force is true, containers and mounted storage will be shut down before
// cleaning up; if force is false, an error will be returned if there are
// still containers running or mounted
func (r *Runtime) Shutdown(force bool) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.valid {
		return ErrRuntimeStopped
	}

	r.valid = false

	// Shutdown all containers if --force is given
	if force {
		ctrs, err := r.state.AllContainers()
		if err != nil {
			logrus.Errorf("Error retrieving containers from database: %v", err)
		} else {
			for _, ctr := range ctrs {
				if err := ctr.Stop(ctrRemoveTimeout); err != nil {
					logrus.Errorf("Error stopping container %s: %v", ctr.ID(), err)
				}
			}
		}
	}

	var lastError error
	if _, err := r.store.Shutdown(force); err != nil {
		lastError = errors.Wrapf(err, "Error shutting down container storage")
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
// TODO: there's a potential race here, where multiple libpods could be in this
// function before the runtime ready file is created
// This probably doesn't matter as the actual container operations are locked
func (r *Runtime) refresh(alivePath string) error {
	// We need to refresh the state of all containers
	ctrs, err := r.state.AllContainers()
	if err != nil {
		return errors.Wrapf(err, "error retrieving all containers from state")
	}
	for _, ctr := range ctrs {
		if err := ctr.refresh(); err != nil {
			return err
		}
	}

	file, err := os.OpenFile(alivePath, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return errors.Wrapf(err, "error creating runtime status file %s", alivePath)
	}
	defer file.Close()

	return nil
}
