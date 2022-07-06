package libpod

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/storage"
	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

const (
	idRegistryName    = "id-registry"
	nameRegistryName  = "name-registry"
	nsRegistryName    = "ns-registry"
	ctrName           = "ctr"
	allCtrsName       = "all-ctrs"
	podName           = "pod"
	allPodsName       = "allPods"
	volName           = "vol"
	allVolsName       = "allVolumes"
	execName          = "exec"
	aliasesName       = "aliases"
	runtimeConfigName = "runtime-config"

	exitCodeName          = "exit-code"
	exitCodeTimeStampName = "exit-code-time-stamp"

	configName         = "config"
	stateName          = "state"
	dependenciesName   = "dependencies"
	volCtrDependencies = "vol-dependencies"
	netNSName          = "netns"
	containersName     = "containers"
	podIDName          = "pod-id"
	namespaceName      = "namespace"
	networksName       = "networks"

	staticDirName   = "static-dir"
	tmpDirName      = "tmp-dir"
	runRootName     = "run-root"
	graphRootName   = "graph-root"
	graphDriverName = "graph-driver-name"
	osName          = "os"
	volPathName     = "volume-path"
)

var (
	idRegistryBkt      = []byte(idRegistryName)
	nameRegistryBkt    = []byte(nameRegistryName)
	nsRegistryBkt      = []byte(nsRegistryName)
	ctrBkt             = []byte(ctrName)
	allCtrsBkt         = []byte(allCtrsName)
	podBkt             = []byte(podName)
	allPodsBkt         = []byte(allPodsName)
	volBkt             = []byte(volName)
	allVolsBkt         = []byte(allVolsName)
	execBkt            = []byte(execName)
	aliasesBkt         = []byte(aliasesName)
	runtimeConfigBkt   = []byte(runtimeConfigName)
	dependenciesBkt    = []byte(dependenciesName)
	volDependenciesBkt = []byte(volCtrDependencies)
	networksBkt        = []byte(networksName)

	exitCodeBkt          = []byte(exitCodeName)
	exitCodeTimeStampBkt = []byte(exitCodeTimeStampName)

	configKey     = []byte(configName)
	stateKey      = []byte(stateName)
	netNSKey      = []byte(netNSName)
	containersBkt = []byte(containersName)
	podIDKey      = []byte(podIDName)
	namespaceKey  = []byte(namespaceName)

	staticDirKey   = []byte(staticDirName)
	tmpDirKey      = []byte(tmpDirName)
	runRootKey     = []byte(runRootName)
	graphRootKey   = []byte(graphRootName)
	graphDriverKey = []byte(graphDriverName)
	osKey          = []byte(osName)
	volPathKey     = []byte(volPathName)
)

// This represents a field in the runtime configuration that will be validated
// against the DB to ensure no configuration mismatches occur.
type dbConfigValidation struct {
	name         string // Only used for error messages
	runtimeValue string
	key          []byte
	defaultValue string
}

// Check if the configuration of the database is compatible with the
// configuration of the runtime opening it
// If there is no runtime configuration loaded, load our own
func checkRuntimeConfig(db *bolt.DB, rt *Runtime) error {
	storeOpts, err := storage.DefaultStoreOptions(rootless.IsRootless(), rootless.GetRootlessUID())
	if err != nil {
		return err
	}

	// We need to validate the following things
	checks := []dbConfigValidation{
		{
			"OS",
			runtime.GOOS,
			osKey,
			runtime.GOOS,
		},
		{
			"libpod root directory (staticdir)",
			filepath.Clean(rt.config.Engine.StaticDir),
			staticDirKey,
			"",
		},
		{
			"libpod temporary files directory (tmpdir)",
			filepath.Clean(rt.config.Engine.TmpDir),
			tmpDirKey,
			"",
		},
		{
			"storage temporary directory (runroot)",
			filepath.Clean(rt.StorageConfig().RunRoot),
			runRootKey,
			storeOpts.RunRoot,
		},
		{
			"storage graph root directory (graphroot)",
			filepath.Clean(rt.StorageConfig().GraphRoot),
			graphRootKey,
			storeOpts.GraphRoot,
		},
		{
			"storage graph driver",
			rt.StorageConfig().GraphDriverName,
			graphDriverKey,
			storeOpts.GraphDriverName,
		},
		{
			"volume path",
			rt.config.Engine.VolumePath,
			volPathKey,
			"",
		},
	}

	// These fields were missing and will have to be recreated.
	missingFields := []dbConfigValidation{}

	// Let's try and validate read-only first
	err = db.View(func(tx *bolt.Tx) error {
		configBkt, err := getRuntimeConfigBucket(tx)
		if err != nil {
			return err
		}

		for _, check := range checks {
			exists, err := readOnlyValidateConfig(configBkt, check)
			if err != nil {
				return err
			}
			if !exists {
				missingFields = append(missingFields, check)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	if len(missingFields) == 0 {
		return nil
	}

	// Populate missing fields
	return db.Update(func(tx *bolt.Tx) error {
		configBkt, err := getRuntimeConfigBucket(tx)
		if err != nil {
			return err
		}

		for _, missing := range missingFields {
			dbValue := []byte(missing.runtimeValue)
			if missing.runtimeValue == "" && missing.defaultValue != "" {
				dbValue = []byte(missing.defaultValue)
			}

			if err := configBkt.Put(missing.key, dbValue); err != nil {
				return fmt.Errorf("error updating %s in DB runtime config: %w", missing.name, err)
			}
		}

		return nil
	})
}

// Attempt a read-only validation of a configuration entry in the DB against an
// element of the current runtime configuration.
// If the configuration key in question does not exist, (false, nil) will be
// returned.
// If the configuration key does exist, and matches the runtime configuration
// successfully, (true, nil) is returned.
// An error is only returned when validation fails.
// if the given runtimeValue or value retrieved from the database are empty,
// and defaultValue is not, defaultValue will be checked instead. This ensures
// that we will not fail on configuration changes in c/storage (where we may
// pass the empty string to use defaults).
func readOnlyValidateConfig(bucket *bolt.Bucket, toCheck dbConfigValidation) (bool, error) {
	keyBytes := bucket.Get(toCheck.key)
	if keyBytes == nil {
		// False return indicates missing key
		return false, nil
	}

	dbValue := string(keyBytes)

	if toCheck.runtimeValue != dbValue {
		// If the runtime value is the empty string and default is not,
		// check against default.
		if toCheck.runtimeValue == "" && toCheck.defaultValue != "" && dbValue == toCheck.defaultValue {
			return true, nil
		}

		// If the DB value is the empty string, check that the runtime
		// value is the default.
		if dbValue == "" && toCheck.defaultValue != "" && toCheck.runtimeValue == toCheck.defaultValue {
			return true, nil
		}

		return true, fmt.Errorf("database %s %q does not match our %s %q: %w",
			toCheck.name, dbValue, toCheck.name, toCheck.runtimeValue, define.ErrDBBadConfig)
	}

	return true, nil
}

// Open a connection to the database.
// Must be paired with a `defer closeDBCon()` on the returned database, to
// ensure the state is properly unlocked
func (s *BoltState) getDBCon() (*bolt.DB, error) {
	// We need an in-memory lock to avoid issues around POSIX file advisory
	// locks as described in the link below:
	// https://www.sqlite.org/src/artifact/c230a7a24?ln=994-1081
	s.dbLock.Lock()

	db, err := bolt.Open(s.dbPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("error opening database %s: %w", s.dbPath, err)
	}

	return db, nil
}

// deferredCloseDBCon closes the bolt db but instead of returning an
// error it logs the error. it is meant to be used within the confines
// of a defer statement only
func (s *BoltState) deferredCloseDBCon(db *bolt.DB) {
	if err := s.closeDBCon(db); err != nil {
		logrus.Errorf("Failed to close libpod db: %q", err)
	}
}

// Close a connection to the database.
// MUST be used in place of `db.Close()` to ensure proper unlocking of the
// state.
func (s *BoltState) closeDBCon(db *bolt.DB) error {
	err := db.Close()

	s.dbLock.Unlock()

	return err
}

func getIDBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(idRegistryBkt)
	if bkt == nil {
		return nil, fmt.Errorf("id registry bucket not found in DB: %w", define.ErrDBBadConfig)
	}
	return bkt, nil
}

func getNamesBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(nameRegistryBkt)
	if bkt == nil {
		return nil, fmt.Errorf("name registry bucket not found in DB: %w", define.ErrDBBadConfig)
	}
	return bkt, nil
}

func getNSBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(nsRegistryBkt)
	if bkt == nil {
		return nil, fmt.Errorf("namespace registry bucket not found in DB: %w", define.ErrDBBadConfig)
	}
	return bkt, nil
}

func getCtrBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(ctrBkt)
	if bkt == nil {
		return nil, fmt.Errorf("containers bucket not found in DB: %w", define.ErrDBBadConfig)
	}
	return bkt, nil
}

func getAllCtrsBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(allCtrsBkt)
	if bkt == nil {
		return nil, fmt.Errorf("all containers bucket not found in DB: %w", define.ErrDBBadConfig)
	}
	return bkt, nil
}

func getPodBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(podBkt)
	if bkt == nil {
		return nil, fmt.Errorf("pods bucket not found in DB: %w", define.ErrDBBadConfig)
	}
	return bkt, nil
}

func getAllPodsBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(allPodsBkt)
	if bkt == nil {
		return nil, fmt.Errorf("all pods bucket not found in DB: %w", define.ErrDBBadConfig)
	}
	return bkt, nil
}

func getVolBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(volBkt)
	if bkt == nil {
		return nil, fmt.Errorf("volumes bucket not found in DB: %w", define.ErrDBBadConfig)
	}
	return bkt, nil
}

func getAllVolsBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(allVolsBkt)
	if bkt == nil {
		return nil, fmt.Errorf("all volumes bucket not found in DB: %w", define.ErrDBBadConfig)
	}
	return bkt, nil
}

func getExecBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(execBkt)
	if bkt == nil {
		return nil, fmt.Errorf("exec bucket not found in DB: %w", define.ErrDBBadConfig)
	}
	return bkt, nil
}

func getRuntimeConfigBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(runtimeConfigBkt)
	if bkt == nil {
		return nil, fmt.Errorf("runtime configuration bucket not found in DB: %w", define.ErrDBBadConfig)
	}
	return bkt, nil
}

func getExitCodeBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(exitCodeBkt)
	if bkt == nil {
		return nil, fmt.Errorf("exit-code container bucket not found in DB: %w", define.ErrDBBadConfig)
	}
	return bkt, nil
}

func getExitCodeTimeStampBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(exitCodeTimeStampBkt)
	if bkt == nil {
		return nil, fmt.Errorf("exit-code time stamp bucket not found in DB: %w", define.ErrDBBadConfig)
	}
	return bkt, nil
}

func (s *BoltState) getContainerConfigFromDB(id []byte, config *ContainerConfig, ctrsBkt *bolt.Bucket) error {
	ctrBkt := ctrsBkt.Bucket(id)
	if ctrBkt == nil {
		return fmt.Errorf("container %s not found in DB: %w", string(id), define.ErrNoSuchCtr)
	}

	if s.namespaceBytes != nil {
		ctrNamespaceBytes := ctrBkt.Get(namespaceKey)
		if !bytes.Equal(s.namespaceBytes, ctrNamespaceBytes) {
			return fmt.Errorf("cannot retrieve container %s as it is part of namespace %q and we are in namespace %q: %w", string(id), string(ctrNamespaceBytes), s.namespace, define.ErrNSMismatch)
		}
	}

	configBytes := ctrBkt.Get(configKey)
	if configBytes == nil {
		return fmt.Errorf("container %s missing config key in DB: %w", string(id), define.ErrInternal)
	}

	if err := json.Unmarshal(configBytes, config); err != nil {
		return fmt.Errorf("error unmarshalling container %s config: %w", string(id), err)
	}

	// convert ports to the new format if needed
	if len(config.ContainerNetworkConfig.OldPortMappings) > 0 && len(config.ContainerNetworkConfig.PortMappings) == 0 {
		config.ContainerNetworkConfig.PortMappings = ocicniPortsToNetTypesPorts(config.ContainerNetworkConfig.OldPortMappings)
		// keep the OldPortMappings in case an user has to downgrade podman

		// indicate the the config was modified and should be written back to the db when possible
		config.rewrite = true
	}

	return nil
}

func (s *BoltState) getContainerFromDB(id []byte, ctr *Container, ctrsBkt *bolt.Bucket) error {
	if err := s.getContainerConfigFromDB(id, ctr.config, ctrsBkt); err != nil {
		return err
	}

	// Get the lock
	lock, err := s.runtime.lockManager.RetrieveLock(ctr.config.LockID)
	if err != nil {
		return fmt.Errorf("error retrieving lock for container %s: %w", string(id), err)
	}
	ctr.lock = lock

	if ctr.config.OCIRuntime == "" {
		ctr.ociRuntime = s.runtime.defaultOCIRuntime
	} else {
		// Handle legacy containers which might use a literal path for
		// their OCI runtime name.
		runtimeName := ctr.config.OCIRuntime
		ociRuntime, ok := s.runtime.ociRuntimes[runtimeName]
		if !ok {
			runtimeSet := false

			// If the path starts with a / and exists, make a new
			// OCI runtime for it using the full path.
			if strings.HasPrefix(runtimeName, "/") {
				if stat, err := os.Stat(runtimeName); err == nil && !stat.IsDir() {
					newOCIRuntime, err := newConmonOCIRuntime(runtimeName, []string{runtimeName}, s.runtime.conmonPath, s.runtime.runtimeFlags, s.runtime.config)
					if err == nil {
						// The runtime lock should
						// protect against concurrent
						// modification of the map.
						ociRuntime = newOCIRuntime
						s.runtime.ociRuntimes[runtimeName] = ociRuntime
						runtimeSet = true
					}
				}
			}

			if !runtimeSet {
				// Use a MissingRuntime implementation
				ociRuntime = getMissingRuntime(runtimeName, s.runtime)
			}
		}
		ctr.ociRuntime = ociRuntime
	}

	ctr.runtime = s.runtime
	ctr.valid = true

	return nil
}

func (s *BoltState) getPodFromDB(id []byte, pod *Pod, podBkt *bolt.Bucket) error {
	podDB := podBkt.Bucket(id)
	if podDB == nil {
		return fmt.Errorf("pod with ID %s not found: %w", string(id), define.ErrNoSuchPod)
	}

	if s.namespaceBytes != nil {
		podNamespaceBytes := podDB.Get(namespaceKey)
		if !bytes.Equal(s.namespaceBytes, podNamespaceBytes) {
			return fmt.Errorf("cannot retrieve pod %s as it is part of namespace %q and we are in namespace %q: %w", string(id), string(podNamespaceBytes), s.namespace, define.ErrNSMismatch)
		}
	}

	podConfigBytes := podDB.Get(configKey)
	if podConfigBytes == nil {
		return fmt.Errorf("pod %s is missing configuration key in DB: %w", string(id), define.ErrInternal)
	}

	if err := json.Unmarshal(podConfigBytes, pod.config); err != nil {
		return fmt.Errorf("error unmarshalling pod %s config from DB: %w", string(id), err)
	}

	// Get the lock
	lock, err := s.runtime.lockManager.RetrieveLock(pod.config.LockID)
	if err != nil {
		return fmt.Errorf("error retrieving lock for pod %s: %w", string(id), err)
	}
	pod.lock = lock

	pod.runtime = s.runtime
	pod.valid = true

	return nil
}

func (s *BoltState) getVolumeFromDB(name []byte, volume *Volume, volBkt *bolt.Bucket) error {
	volDB := volBkt.Bucket(name)
	if volDB == nil {
		return fmt.Errorf("volume with name %s not found: %w", string(name), define.ErrNoSuchVolume)
	}

	volConfigBytes := volDB.Get(configKey)
	if volConfigBytes == nil {
		return fmt.Errorf("volume %s is missing configuration key in DB: %w", string(name), define.ErrInternal)
	}

	if err := json.Unmarshal(volConfigBytes, volume.config); err != nil {
		return fmt.Errorf("error unmarshalling volume %s config from DB: %w", string(name), err)
	}

	// Volume state is allowed to be nil for legacy compatibility
	volStateBytes := volDB.Get(stateKey)
	if volStateBytes != nil {
		if err := json.Unmarshal(volStateBytes, volume.state); err != nil {
			return fmt.Errorf("error unmarshalling volume %s state from DB: %w", string(name), err)
		}
	}

	// Retrieve volume driver
	if volume.UsesVolumeDriver() {
		plugin, err := s.runtime.getVolumePlugin(volume.config)
		if err != nil {
			// We want to fail gracefully here, to ensure that we
			// can still remove volumes even if their plugin is
			// missing. Otherwise, we end up with volumes that
			// cannot even be retrieved from the database and will
			// cause things like `volume ls` to fail.
			logrus.Errorf("Volume %s uses volume plugin %s, but it cannot be accessed - some functionality may not be available: %v", volume.Name(), volume.config.Driver, err)
		} else {
			volume.plugin = plugin
		}
	}

	// Get the lock
	lock, err := s.runtime.lockManager.RetrieveLock(volume.config.LockID)
	if err != nil {
		return fmt.Errorf("error retrieving lock for volume %q: %w", string(name), err)
	}
	volume.lock = lock

	volume.runtime = s.runtime
	volume.valid = true

	return nil
}

// Add a container to the DB
// If pod is not nil, the container is added to the pod as well
func (s *BoltState) addContainer(ctr *Container, pod *Pod) error {
	if s.namespace != "" && s.namespace != ctr.config.Namespace {
		return fmt.Errorf("cannot add container %s as it is in namespace %q and we are in namespace %q: %w",
			ctr.ID(), s.namespace, ctr.config.Namespace, define.ErrNSMismatch)
	}

	// Set the original networks to nil. We can save some space by not storing it in the config
	// since we store it in a different mutable bucket anyway.
	configNetworks := ctr.config.Networks
	ctr.config.Networks = nil

	// JSON container structs to insert into DB
	configJSON, err := json.Marshal(ctr.config)
	if err != nil {
		return fmt.Errorf("error marshalling container %s config to JSON: %w", ctr.ID(), err)
	}
	stateJSON, err := json.Marshal(ctr.state)
	if err != nil {
		return fmt.Errorf("error marshalling container %s state to JSON: %w", ctr.ID(), err)
	}
	netNSPath := getNetNSPath(ctr)
	dependsCtrs := ctr.Dependencies()

	ctrID := []byte(ctr.ID())
	ctrName := []byte(ctr.Name())

	var ctrNamespace []byte
	if ctr.config.Namespace != "" {
		ctrNamespace = []byte(ctr.config.Namespace)
	}

	// make sure to marshal the network options before we get the db lock
	networks := make(map[string][]byte, len(configNetworks))
	for net, opts := range configNetworks {
		// Check that we don't have any empty network names
		if net == "" {
			return fmt.Errorf("network names cannot be an empty string: %w", define.ErrInvalidArg)
		}
		if opts.InterfaceName == "" {
			return fmt.Errorf("network interface name cannot be an empty string: %w", define.ErrInvalidArg)
		}
		// always add the short id as alias for docker compat
		opts.Aliases = append(opts.Aliases, ctr.config.ID[:12])
		optBytes, err := json.Marshal(opts)
		if err != nil {
			return fmt.Errorf("error marshalling network options JSON for container %s: %w", ctr.ID(), err)
		}
		networks[net] = optBytes
	}

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	err = db.Update(func(tx *bolt.Tx) error {
		idsBucket, err := getIDBucket(tx)
		if err != nil {
			return err
		}

		namesBucket, err := getNamesBucket(tx)
		if err != nil {
			return err
		}

		nsBucket, err := getNSBucket(tx)
		if err != nil {
			return err
		}

		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		allCtrsBucket, err := getAllCtrsBucket(tx)
		if err != nil {
			return err
		}

		volBkt, err := getVolBucket(tx)
		if err != nil {
			return err
		}

		// If a pod was given, check if it exists
		var podDB *bolt.Bucket
		var podCtrs *bolt.Bucket
		if pod != nil {
			podBucket, err := getPodBucket(tx)
			if err != nil {
				return err
			}

			podID := []byte(pod.ID())

			podDB = podBucket.Bucket(podID)
			if podDB == nil {
				pod.valid = false
				return fmt.Errorf("pod %s does not exist in database: %w", pod.ID(), define.ErrNoSuchPod)
			}
			podCtrs = podDB.Bucket(containersBkt)
			if podCtrs == nil {
				return fmt.Errorf("pod %s does not have a containers bucket: %w", pod.ID(), define.ErrInternal)
			}

			podNS := podDB.Get(namespaceKey)
			if !bytes.Equal(podNS, ctrNamespace) {
				return fmt.Errorf("container %s is in namespace %s and pod %s is in namespace %s: %w",
					ctr.ID(), ctr.config.Namespace, pod.ID(), pod.config.Namespace, define.ErrNSMismatch)
			}
		}

		// Check if we already have a container with the given ID and name
		idExist := idsBucket.Get(ctrID)
		if idExist != nil {
			err = define.ErrCtrExists
			if allCtrsBucket.Get(idExist) == nil {
				err = define.ErrPodExists
			}
			return fmt.Errorf("ID \"%s\" is in use: %w", ctr.ID(), err)
		}
		nameExist := namesBucket.Get(ctrName)
		if nameExist != nil {
			err = define.ErrCtrExists
			if allCtrsBucket.Get(nameExist) == nil {
				err = define.ErrPodExists
			}
			return fmt.Errorf("name \"%s\" is in use: %w", ctr.Name(), err)
		}

		// No overlapping containers
		// Add the new container to the DB
		if err := idsBucket.Put(ctrID, ctrName); err != nil {
			return fmt.Errorf("error adding container %s ID to DB: %w", ctr.ID(), err)
		}
		if err := namesBucket.Put(ctrName, ctrID); err != nil {
			return fmt.Errorf("error adding container %s name (%s) to DB: %w", ctr.ID(), ctr.Name(), err)
		}
		if ctrNamespace != nil {
			if err := nsBucket.Put(ctrID, ctrNamespace); err != nil {
				return fmt.Errorf("error adding container %s namespace (%q) to DB: %w", ctr.ID(), ctr.Namespace(), err)
			}
		}
		if err := allCtrsBucket.Put(ctrID, ctrName); err != nil {
			return fmt.Errorf("error adding container %s to all containers bucket in DB: %w", ctr.ID(), err)
		}

		newCtrBkt, err := ctrBucket.CreateBucket(ctrID)
		if err != nil {
			return fmt.Errorf("error adding container %s bucket to DB: %w", ctr.ID(), err)
		}

		if err := newCtrBkt.Put(configKey, configJSON); err != nil {
			return fmt.Errorf("error adding container %s config to DB: %w", ctr.ID(), err)
		}
		if err := newCtrBkt.Put(stateKey, stateJSON); err != nil {
			return fmt.Errorf("error adding container %s state to DB: %w", ctr.ID(), err)
		}
		if ctrNamespace != nil {
			if err := newCtrBkt.Put(namespaceKey, ctrNamespace); err != nil {
				return fmt.Errorf("error adding container %s namespace to DB: %w", ctr.ID(), err)
			}
		}
		if pod != nil {
			if err := newCtrBkt.Put(podIDKey, []byte(pod.ID())); err != nil {
				return fmt.Errorf("error adding container %s pod to DB: %w", ctr.ID(), err)
			}
		}
		if netNSPath != "" {
			if err := newCtrBkt.Put(netNSKey, []byte(netNSPath)); err != nil {
				return fmt.Errorf("error adding container %s netns path to DB: %w", ctr.ID(), err)
			}
		}
		if len(networks) > 0 {
			ctrNetworksBkt, err := newCtrBkt.CreateBucket(networksBkt)
			if err != nil {
				return fmt.Errorf("error creating networks bucket for container %s: %w", ctr.ID(), err)
			}
			for network, opts := range networks {
				if err := ctrNetworksBkt.Put([]byte(network), opts); err != nil {
					return fmt.Errorf("error adding network %q to networks bucket for container %s: %w", network, ctr.ID(), err)
				}
			}
		}

		if _, err := newCtrBkt.CreateBucket(dependenciesBkt); err != nil {
			return fmt.Errorf("error creating dependencies bucket for container %s: %w", ctr.ID(), err)
		}

		// Add dependencies for the container
		for _, dependsCtr := range dependsCtrs {
			depCtrID := []byte(dependsCtr)

			depCtrBkt := ctrBucket.Bucket(depCtrID)
			if depCtrBkt == nil {
				return fmt.Errorf("container %s depends on container %s, but it does not exist in the DB: %w", ctr.ID(), dependsCtr, define.ErrNoSuchCtr)
			}

			depCtrPod := depCtrBkt.Get(podIDKey)
			if pod != nil {
				// If we're part of a pod, make sure the dependency is part of the same pod
				if depCtrPod == nil {
					return fmt.Errorf("container %s depends on container %s which is not in pod %s: %w", ctr.ID(), dependsCtr, pod.ID(), define.ErrInvalidArg)
				}

				if string(depCtrPod) != pod.ID() {
					return fmt.Errorf("container %s depends on container %s which is in a different pod (%s): %w", ctr.ID(), dependsCtr, string(depCtrPod), define.ErrInvalidArg)
				}
			} else if depCtrPod != nil {
				// If we're not part of a pod, we cannot depend on containers in a pod
				return fmt.Errorf("container %s depends on container %s which is in a pod - containers not in pods cannot depend on containers in pods: %w", ctr.ID(), dependsCtr, define.ErrInvalidArg)
			}

			depNamespace := depCtrBkt.Get(namespaceKey)
			if !bytes.Equal(ctrNamespace, depNamespace) {
				return fmt.Errorf("container %s in namespace %q depends on container %s in namespace %q - namespaces must match: %w", ctr.ID(), ctr.config.Namespace, dependsCtr, string(depNamespace), define.ErrNSMismatch)
			}

			depCtrDependsBkt := depCtrBkt.Bucket(dependenciesBkt)
			if depCtrDependsBkt == nil {
				return fmt.Errorf("container %s does not have a dependencies bucket: %w", dependsCtr, define.ErrInternal)
			}
			if err := depCtrDependsBkt.Put(ctrID, ctrName); err != nil {
				return fmt.Errorf("error adding ctr %s as dependency of container %s: %w", ctr.ID(), dependsCtr, err)
			}
		}

		// Add ctr to pod
		if pod != nil && podCtrs != nil {
			if err := podCtrs.Put(ctrID, ctrName); err != nil {
				return fmt.Errorf("error adding container %s to pod %s: %w", ctr.ID(), pod.ID(), err)
			}
		}

		// Add container to named volume dependencies buckets
		for _, vol := range ctr.config.NamedVolumes {
			volDB := volBkt.Bucket([]byte(vol.Name))
			if volDB == nil {
				return fmt.Errorf("no volume with name %s found in database when adding container %s: %w", vol.Name, ctr.ID(), define.ErrNoSuchVolume)
			}

			ctrDepsBkt, err := volDB.CreateBucketIfNotExists(volDependenciesBkt)
			if err != nil {
				return fmt.Errorf("error creating volume %s dependencies bucket to add container %s: %w", vol.Name, ctr.ID(), err)
			}
			if depExists := ctrDepsBkt.Get(ctrID); depExists == nil {
				if err := ctrDepsBkt.Put(ctrID, ctrID); err != nil {
					return fmt.Errorf("error adding container %s to volume %s dependencies: %w", ctr.ID(), vol.Name, err)
				}
			}
		}

		return nil
	})
	return err
}

// Remove a container from the DB
// If pod is not nil, the container is treated as belonging to a pod, and
// will be removed from the pod as well
func (s *BoltState) removeContainer(ctr *Container, pod *Pod, tx *bolt.Tx) error {
	ctrID := []byte(ctr.ID())
	ctrName := []byte(ctr.Name())

	idsBucket, err := getIDBucket(tx)
	if err != nil {
		return err
	}

	namesBucket, err := getNamesBucket(tx)
	if err != nil {
		return err
	}

	ctrBucket, err := getCtrBucket(tx)
	if err != nil {
		return err
	}

	nsBucket, err := getNSBucket(tx)
	if err != nil {
		return err
	}

	allCtrsBucket, err := getAllCtrsBucket(tx)
	if err != nil {
		return err
	}

	volBkt, err := getVolBucket(tx)
	if err != nil {
		return err
	}

	// Does the pod exist?
	var podDB *bolt.Bucket
	if pod != nil {
		podBucket, err := getPodBucket(tx)
		if err != nil {
			return err
		}

		podID := []byte(pod.ID())

		podDB = podBucket.Bucket(podID)
		if podDB == nil {
			pod.valid = false
			return fmt.Errorf("no pod with ID %s found in DB: %w", pod.ID(), define.ErrNoSuchPod)
		}
	}

	// Does the container exist?
	ctrExists := ctrBucket.Bucket(ctrID)
	if ctrExists == nil {
		ctr.valid = false
		return fmt.Errorf("no container with ID %s found in DB: %w", ctr.ID(), define.ErrNoSuchCtr)
	}

	// Compare namespace
	// We can't remove containers not in our namespace
	if s.namespace != "" {
		if s.namespace != ctr.config.Namespace {
			return fmt.Errorf("container %s is in namespace %q, does not match our namespace %q: %w", ctr.ID(), ctr.config.Namespace, s.namespace, define.ErrNSMismatch)
		}
		if pod != nil && s.namespace != pod.config.Namespace {
			return fmt.Errorf("pod %s is in namespace %q, does not match out namespace %q: %w", pod.ID(), pod.config.Namespace, s.namespace, define.ErrNSMismatch)
		}
	}

	if podDB != nil && pod != nil {
		// Check if the container is in the pod, remove it if it is
		podCtrs := podDB.Bucket(containersBkt)
		if podCtrs == nil {
			// Malformed pod
			logrus.Errorf("Pod %s malformed in database, missing containers bucket!", pod.ID())
		} else {
			ctrInPod := podCtrs.Get(ctrID)
			if ctrInPod == nil {
				return fmt.Errorf("container %s is not in pod %s: %w", ctr.ID(), pod.ID(), define.ErrNoSuchCtr)
			}
			if err := podCtrs.Delete(ctrID); err != nil {
				return fmt.Errorf("error removing container %s from pod %s: %w", ctr.ID(), pod.ID(), err)
			}
		}
	}

	// Does the container have exec sessions?
	ctrExecSessionsBkt := ctrExists.Bucket(execBkt)
	if ctrExecSessionsBkt != nil {
		sessions := []string{}
		err = ctrExecSessionsBkt.ForEach(func(id, value []byte) error {
			sessions = append(sessions, string(id))

			return nil
		})
		if err != nil {
			return err
		}
		if len(sessions) > 0 {
			return fmt.Errorf("container %s has active exec sessions: %s: %w", ctr.ID(), strings.Join(sessions, ", "), define.ErrExecSessionExists)
		}
	}

	// Does the container have dependencies?
	ctrDepsBkt := ctrExists.Bucket(dependenciesBkt)
	if ctrDepsBkt == nil {
		return fmt.Errorf("container %s does not have a dependencies bucket: %w", ctr.ID(), define.ErrInternal)
	}
	deps := []string{}
	err = ctrDepsBkt.ForEach(func(id, value []byte) error {
		deps = append(deps, string(id))

		return nil
	})
	if err != nil {
		return err
	}
	if len(deps) != 0 {
		return fmt.Errorf("container %s is a dependency of the following containers: %s: %w", ctr.ID(), strings.Join(deps, ", "), define.ErrDepExists)
	}

	if err := ctrBucket.DeleteBucket(ctrID); err != nil {
		return fmt.Errorf("error deleting container %s from DB: %w", ctr.ID(), define.ErrInternal)
	}

	if err := idsBucket.Delete(ctrID); err != nil {
		return fmt.Errorf("error deleting container %s ID in DB: %w", ctr.ID(), err)
	}

	if err := namesBucket.Delete(ctrName); err != nil {
		return fmt.Errorf("error deleting container %s name in DB: %w", ctr.ID(), err)
	}
	if err := nsBucket.Delete(ctrID); err != nil {
		return fmt.Errorf("error deleting container %s namespace in DB: %w", ctr.ID(), err)
	}
	if err := allCtrsBucket.Delete(ctrID); err != nil {
		return fmt.Errorf("error deleting container %s from all containers bucket in DB: %w", ctr.ID(), err)
	}

	depCtrs := ctr.Dependencies()

	// Remove us from other container's dependencies
	for _, depCtr := range depCtrs {
		depCtrID := []byte(depCtr)

		depCtrBkt := ctrBucket.Bucket(depCtrID)
		if depCtrBkt == nil {
			// The dependent container has been removed
			// This should not be possible, and means the
			// state is inconsistent, but don't error
			// The container with inconsistent state is the
			// one being removed
			continue
		}

		depCtrDependsBkt := depCtrBkt.Bucket(dependenciesBkt)
		if depCtrDependsBkt == nil {
			// This is more serious - another container in
			// the state is inconsistent
			// Log it, continue removing
			logrus.Errorf("Container %s is missing dependencies bucket in DB", ctr.ID())
			continue
		}

		if err := depCtrDependsBkt.Delete(ctrID); err != nil {
			return fmt.Errorf("error removing container %s as a dependency of container %s: %w", ctr.ID(), depCtr, err)
		}
	}

	// Remove container from named volume dependencies buckets
	for _, vol := range ctr.config.NamedVolumes {
		volDB := volBkt.Bucket([]byte(vol.Name))
		if volDB == nil {
			// Let's assume the volume was already deleted and
			// continue to remove the container
			continue
		}

		ctrDepsBkt := volDB.Bucket(volDependenciesBkt)
		if ctrDepsBkt == nil {
			return fmt.Errorf("volume %s is missing container dependencies bucket, cannot remove container %s from dependencies: %w", vol.Name, ctr.ID(), define.ErrInternal)
		}
		if depExists := ctrDepsBkt.Get(ctrID); depExists == nil {
			if err := ctrDepsBkt.Delete(ctrID); err != nil {
				return fmt.Errorf("error deleting container %s dependency on volume %s: %w", ctr.ID(), vol.Name, err)
			}
		}
	}

	return nil
}

// lookupContainerID retrieves a container ID from the state by full or unique
// partial ID or name.
// NOTE: the retrieved container ID namespace may not match the state namespace.
func (s *BoltState) lookupContainerID(idOrName string, ctrBucket, namesBucket, nsBucket *bolt.Bucket) ([]byte, error) {
	// First, check if the ID given was the actual container ID
	ctrExists := ctrBucket.Bucket([]byte(idOrName))
	if ctrExists != nil {
		// A full container ID was given.
		// It might not be in our namespace, but this will be handled
		// the callers.
		return []byte(idOrName), nil
	}

	// Next, check if the full name was given
	isPod := false
	fullID := namesBucket.Get([]byte(idOrName))
	if fullID != nil {
		// The name exists and maps to an ID.
		// However, we are not yet certain the ID is a
		// container.
		ctrExists = ctrBucket.Bucket(fullID)
		if ctrExists != nil {
			// A container bucket matching the full ID was
			// found.
			return fullID, nil
		}
		// Don't error if we have a name match but it's not a
		// container - there's a chance we have a container with
		// an ID starting with those characters.
		// However, so we can return a good error, note whether
		// this is a pod.
		isPod = true
	}

	var id []byte
	// We were not given a full container ID or name.
	// Search for partial ID matches.
	exists := false
	err := ctrBucket.ForEach(func(checkID, checkName []byte) error {
		// If the container isn't in our namespace, we
		// can't match it
		if s.namespaceBytes != nil {
			ns := nsBucket.Get(checkID)
			if !bytes.Equal(ns, s.namespaceBytes) {
				return nil
			}
		}
		if strings.HasPrefix(string(checkID), idOrName) {
			if exists {
				return fmt.Errorf("more than one result for container ID %s: %w", idOrName, define.ErrCtrExists)
			}
			id = checkID
			exists = true
		}

		return nil
	})

	if err != nil {
		return nil, err
	} else if !exists {
		if isPod {
			return nil, fmt.Errorf("%q is a pod, not a container: %w", idOrName, define.ErrNoSuchCtr)
		}
		return nil, fmt.Errorf("no container with name or ID %q found: %w", idOrName, define.ErrNoSuchCtr)
	}
	return id, nil
}
