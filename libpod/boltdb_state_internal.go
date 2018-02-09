package libpod

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	idRegistryName    = "id-registry"
	nameRegistryName  = "name-registry"
	ctrName           = "ctr"
	podName           = "pod"
	runtimeConfigName = "runtime-config"

	configName       = "config"
	stateName        = "state"
	dependenciesName = "dependencies"
	netNSName        = "netns"
	containersName   = "containers"

	dbExistName = "ok"
)

var (
	idRegistryBkt    = []byte(idRegistryName)
	nameRegistryBkt  = []byte(nameRegistryName)
	ctrBkt           = []byte(ctrName)
	podBkt           = []byte(podName)
	runtimeConfigBkt = []byte(runtimeConfigName)

	configKey       = []byte(configName)
	stateKey        = []byte(stateName)
	dependenciesBkt = []byte(dependenciesName)
	netNSKey        = []byte(netNSName)
	containersBkt   = []byte(containersName)
)

// Check if the configuration of the database is compatible with the
// configuration of the runtime opening it
// If there is no runtime configuration loaded, load our own
func checkRuntimeConfig(db *bolt.DB, runtime *Runtime) error {
	var (
		staticDir       = []byte("static-dir")
		tmpDir          = []byte("tmp-dir")
		runRoot         = []byte("run-root")
		graphRoot       = []byte("graph-root")
		graphDriverName = []byte("graph-driver-name")
	)

	err := db.Update(func(tx *bolt.Tx) error {
		configBkt, err := getRuntimeConfigBucket(tx)
		if err != nil {
			return err
		}

		if err := validateDBAgainstConfig(configBkt, "static dir",
			runtime.config.StaticDir, staticDir); err != nil {
			return err
		}

		if err := validateDBAgainstConfig(configBkt, "tmp dir",
			runtime.config.TmpDir, tmpDir); err != nil {
			return err
		}

		if err := validateDBAgainstConfig(configBkt, "run root",
			runtime.config.StorageConfig.RunRoot, runRoot); err != nil {
			return err
		}

		if err := validateDBAgainstConfig(configBkt, "graph root",
			runtime.config.StorageConfig.GraphRoot, graphRoot); err != nil {
			return err
		}

		return validateDBAgainstConfig(configBkt, "graph driver name",
			runtime.config.StorageConfig.GraphDriverName,
			graphDriverName)
	})

	return err
}

// Validate a configuration entry in the DB against current runtime config
// If the given configuration key does not exist it will be created
func validateDBAgainstConfig(bucket *bolt.Bucket, fieldName, runtimeValue string, keyName []byte) error {
	keyBytes := bucket.Get(keyName)
	if keyBytes == nil {
		if err := bucket.Put(keyName, []byte(runtimeValue)); err != nil {
			return errors.Wrapf(err, "error updating %s in DB runtime config", fieldName)
		}
	} else {
		if runtimeValue != string(keyBytes) {
			return errors.Wrapf(ErrDBBadConfig, "database %s %s does not match our %s %s",
				fieldName, string(keyBytes), fieldName, runtimeValue)
		}
	}

	return nil
}

func (s *BoltState) getDBCon() (*bolt.DB, error) {
	db, err := bolt.Open(s.dbPath, 0600, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "error opening database %s", s.dbPath)
	}

	return db, nil
}

func getIDBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(idRegistryBkt)
	if bkt == nil {
		return nil, errors.Wrapf(ErrDBBadConfig, "id registry bucket not found in DB")
	}
	return bkt, nil
}

func getNamesBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(nameRegistryBkt)
	if bkt == nil {
		return nil, errors.Wrapf(ErrDBBadConfig, "name registry bucket not found in DB")
	}
	return bkt, nil
}

func getCtrBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(ctrBkt)
	if bkt == nil {
		return nil, errors.Wrapf(ErrDBBadConfig, "containers bucket not found in DB")
	}
	return bkt, nil
}

func getPodBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(podBkt)
	if bkt == nil {
		return nil, errors.Wrapf(ErrDBBadConfig, "pods bucket not found in DB")
	}
	return bkt, nil
}

func getRuntimeConfigBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(runtimeConfigBkt)
	if bkt == nil {
		return nil, errors.Wrapf(ErrDBBadConfig, "runtime configuration bucket not found in DB")
	}
	return bkt, nil
}

func (s *BoltState) getContainerFromDB(id []byte, ctr *Container, ctrsBkt *bolt.Bucket) error {
	ctrBkt := ctrsBkt.Bucket(id)
	if ctrBkt == nil {
		return errors.Wrapf(ErrNoSuchCtr, "container %s not found in DB", string(id))
	}

	configBytes := ctrBkt.Get(configKey)
	if configBytes == nil {
		return errors.Wrapf(ErrInternal, "container %s missing config key in DB", string(id))
	}

	stateBytes := ctrBkt.Get(stateKey)
	if stateBytes == nil {
		return errors.Wrapf(ErrInternal, "container %s missing state key in DB", string(id))
	}

	netNSBytes := ctrBkt.Get(netNSKey)

	if err := json.Unmarshal(configBytes, ctr.config); err != nil {
		return errors.Wrapf(err, "error unmarshalling container %s config", string(id))
	}

	if err := json.Unmarshal(stateBytes, ctr.state); err != nil {
		return errors.Wrapf(err, "error unmarshalling container %s state", string(id))
	}

	// The container may not have a network namespace, so it's OK if this is
	// nil
	if netNSBytes != nil {
		nsPath := string(netNSBytes)
		netNS, err := joinNetNS(nsPath)
		if err != nil {
			return errors.Wrapf(err, "error joining network namespace for container %s", string(id))
		}
		ctr.state.NetNS = netNS
	}

	// Get the lock
	lockPath := filepath.Join(s.lockDir, string(id))
	lock, err := storage.GetLockfile(lockPath)
	if err != nil {
		return errors.Wrapf(err, "error retrieving lockfile for container %s", string(id))
	}
	ctr.lock = lock

	ctr.runtime = s.runtime
	ctr.valid = true

	return nil
}

func (s *BoltState) getPodFromDB(id []byte, pod *Pod, podBkt *bolt.Bucket) error {
	podDB := podBkt.Bucket(id)
	if podDB == nil {
		return errors.Wrapf(ErrNoSuchPod, "pod with ID %s not found", string(id))
	}

	podBytes := podDB.Get(configKey)
	if podBytes == nil {
		return errors.Wrapf(ErrInternal, "pod %s is missing configuration key in DB", string(id))
	}

	if err := json.Unmarshal(podBytes, pod.config); err != nil {
		return errors.Wrapf(err, "error unmarshalling pod %s from DB", string(id))
	}

	// Get the lock
	lockPath := filepath.Join(s.lockDir, string(id))
	lock, err := storage.GetLockfile(lockPath)
	if err != nil {
		return errors.Wrapf(err, "error retrieving lockfile for pod %s", string(id))
	}
	pod.lock = lock

	pod.runtime = s.runtime
	pod.valid = true

	return nil
}

// Add a container to the DB
// If pod is not nil, the container is added to the pod as well
func (s *BoltState) addContainer(ctr *Container, pod *Pod) error {
	// JSON container structs to insert into DB
	// TODO use a higher-performance struct encoding than JSON
	configJSON, err := json.Marshal(ctr.config)
	if err != nil {
		return errors.Wrapf(err, "error marshalling container %s config to JSON", ctr.ID())
	}
	stateJSON, err := json.Marshal(ctr.state)
	if err != nil {
		return errors.Wrapf(err, "error marshalling container %s state to JSON", ctr.ID())
	}
	netNSPath := ""
	if ctr.state.NetNS != nil {
		netNSPath = ctr.state.NetNS.Path()
	}

	dependsCtrs := ctr.Dependencies()

	ctrID := []byte(ctr.ID())
	ctrName := []byte(ctr.Name())

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
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
				return errors.Wrapf(ErrNoSuchPod, "pod %s does not exist in database", pod.ID())
			}
			podCtrs = podDB.Bucket(containersBkt)
			if podCtrs == nil {
				return errors.Wrapf(ErrInternal, "pod %s does not have a containers bucket", pod.ID())
			}
		}

		// Check if we already have a container with the given ID and name
		idExist := idsBucket.Get(ctrID)
		if idExist != nil {
			return errors.Wrapf(ErrCtrExists, "ID %s is in use", ctr.ID())
		}
		nameExist := namesBucket.Get(ctrName)
		if nameExist != nil {
			return errors.Wrapf(ErrCtrExists, "name %s is in use", ctr.Name())
		}

		// No overlapping containers
		// Add the new container to the DB
		if err := idsBucket.Put(ctrID, ctrName); err != nil {
			return errors.Wrapf(err, "error adding container %s ID to DB", ctr.ID())
		}
		if err := namesBucket.Put(ctrName, ctrID); err != nil {
			return errors.Wrapf(err, "error adding container %s name (%s) to DB", ctr.ID(), ctr.Name())
		}

		newCtrBkt, err := ctrBucket.CreateBucket(ctrID)
		if err != nil {
			return errors.Wrapf(err, "error adding container %s bucket to DB", ctr.ID())
		}

		if err := newCtrBkt.Put(configKey, configJSON); err != nil {
			return errors.Wrapf(err, "error adding container %s config to DB", ctr.ID())
		}
		if err := newCtrBkt.Put(stateKey, stateJSON); err != nil {
			return errors.Wrapf(err, "error adding container %s state to DB", ctr.ID())
		}
		if netNSPath != "" {
			if err := newCtrBkt.Put(netNSKey, []byte(netNSPath)); err != nil {
				return errors.Wrapf(err, "error adding container %s netns path to DB", ctr.ID())
			}
		}
		if _, err := newCtrBkt.CreateBucket(dependenciesBkt); err != nil {
			return errors.Wrapf(err, "error creating dependencies bucket for container %s", ctr.ID())
		}

		// Add dependencies for the container
		for _, dependsCtr := range dependsCtrs {
			depCtrID := []byte(dependsCtr)

			depCtrBkt := ctrBucket.Bucket(depCtrID)
			if depCtrBkt == nil {
				return errors.Wrapf(ErrNoSuchCtr, "container %s depends on container %s, but it does not exist in the DB", ctr.ID(), dependsCtr)
			}
			depCtrDependsBkt := depCtrBkt.Bucket(dependenciesBkt)
			if depCtrDependsBkt == nil {
				return errors.Wrapf(ErrInternal, "container %s does not have a dependencies bucket", dependsCtr)
			}
			if err := depCtrDependsBkt.Put(ctrID, ctrName); err != nil {
				return errors.Wrapf(err, "error adding ctr %s as dependency of container %s", ctr.ID(), dependsCtr)
			}
		}

		// Add ctr to pod
		if pod != nil {
			if err := podCtrs.Put(ctrID, ctrName); err != nil {
				return errors.Wrapf(err, "error adding container %s to pod %s", ctr.ID(), pod.ID())
			}
		}

		return nil
	})
	return err
}

// Remove a container from the DB
// If pod is not nil, the container is treated as belonging to a pod, and
// will be removed from the pod as well
func removeContainer(ctr *Container, pod *Pod, tx *bolt.Tx) error {
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
			return errors.Wrapf(ErrNoSuchPod, "no pod with ID %s found in DB", pod.ID())
		}
	}

	// Does the container exist?
	ctrExists := ctrBucket.Bucket(ctrID)
	if ctrExists == nil {
		ctr.valid = false
		return errors.Wrapf(ErrNoSuchCtr, "no container with ID %s found in DB", ctr.ID())
	}

	if podDB != nil {
		// Check if the container is in the pod, remove it if it is
		podCtrs := podDB.Bucket(containersBkt)
		if podCtrs == nil {
			// Malformed pod
			logrus.Errorf("pod %s malformed in database, missing containers bucket!", pod.ID())
		} else {
			ctrInPod := podCtrs.Get(ctrID)
			if ctrInPod == nil {
				return errors.Wrapf(ErrNoSuchCtr, "container %s is not in pod %s", ctr.ID(), pod.ID())
			}
			if err := podCtrs.Delete(ctrID); err != nil {
				return errors.Wrapf(err, "error removing container %s from pod %s", ctr.ID(), pod.ID())
			}
		}
	}

	// Does the container have dependencies?
	ctrDepsBkt := ctrExists.Bucket(dependenciesBkt)
	if ctrDepsBkt == nil {
		return errors.Wrapf(ErrInternal, "container %s does not have a dependencies bucket", ctr.ID())
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
		return errors.Wrapf(ErrCtrExists, "container %s is a dependency of the following containers: %s", ctr.ID(), strings.Join(deps, ", "))
	}

	if err := ctrBucket.DeleteBucket(ctrID); err != nil {
		return errors.Wrapf(ErrInternal, "error deleting container %s from DB", ctr.ID())
	}

	if err := idsBucket.Delete(ctrID); err != nil {
		return errors.Wrapf(err, "error deleting container %s ID in DB", ctr.ID())
	}

	if err := namesBucket.Delete(ctrName); err != nil {
		return errors.Wrapf(err, "error deleting container %s name in DB", ctr.ID())
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
			return errors.Wrapf(err, "error removing container %s as a dependency of container %s", ctr.ID(), depCtr)
		}
	}

	return nil
}
