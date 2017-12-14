package libpod

import (
	"encoding/json"
	"path/filepath"

	"github.com/boltdb/bolt"
	"github.com/containers/storage"
	"github.com/pkg/errors"
)

const (
	idRegistryName    = "id-registry"
	nameRegistryName  = "name-registry"
	ctrConfigName     = "container-config"
	ctrStateName      = "container-state"
	netNSName         = "net-ns"
	runtimeConfigName = "runtime-config"
	ctrDependsName    = "container-depends"
)

var (
	idRegistryBkt    = []byte(idRegistryName)
	nameRegistryBkt  = []byte(nameRegistryName)
	ctrConfigBkt     = []byte(ctrConfigName)
	ctrStateBkt      = []byte(ctrStateName)
	netNSBkt         = []byte(netNSName)
	runtimeConfigBkt = []byte(runtimeConfigName)
	ctrDependsBkt    = []byte(ctrDependsName)
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

func getCtrConfigBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(ctrConfigBkt)
	if bkt == nil {
		return nil, errors.Wrapf(ErrDBBadConfig, "container config bucket not found in DB")
	}
	return bkt, nil
}

func getCtrStateBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(ctrStateBkt)
	if bkt == nil {
		return nil, errors.Wrapf(ErrDBBadConfig, "container state bucket not found in DB")
	}
	return bkt, nil
}

func getNetNSBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(netNSBkt)
	if bkt == nil {
		return nil, errors.Wrapf(ErrDBBadConfig, "network namespace bucket not found in DB")
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

func getCtrDependsBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(ctrDependsBkt)
	if bkt == nil {
		return nil, errors.Wrapf(ErrDBBadConfig, "container dependencies bucket not found in DB")
	}
	return bkt, nil
}

func (s *BoltState) getContainerFromDB(id []byte, ctr *Container, config, state, netNS *bolt.Bucket) error {
	configBytes := config.Get(id)
	if configBytes == nil {
		return errors.Wrapf(ErrNoSuchCtr, "error unmarshalling container %s config", string(id))
	}

	if err := json.Unmarshal(configBytes, ctr.config); err != nil {
		return errors.Wrapf(err, "error unmarshalling container %s config", string(id))
	}

	stateBytes := state.Get(id)
	if stateBytes == nil {
		return errors.Wrapf(ErrInternal, "container %s has config but no state", string(id))
	}

	if err := json.Unmarshal(stateBytes, ctr.state); err != nil {
		return errors.Wrapf(err, "error unmarshalling container %s state", string(id))
	}

	// The container may not have a network namespace, so it's OK if this is
	// nil
	netNSBytes := netNS.Get(id)
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
