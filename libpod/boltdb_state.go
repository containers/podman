package libpod

import (
	"bytes"
	"os"
	"strings"
	"sync"

	"github.com/containers/podman/v2/libpod/define"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// BoltState is a state implementation backed by a Bolt DB
type BoltState struct {
	valid          bool
	dbPath         string
	dbLock         sync.Mutex
	namespace      string
	namespaceBytes []byte
	runtime        *Runtime
}

// A brief description of the format of the BoltDB state:
// At the top level, the following buckets are created:
// - idRegistryBkt: Maps ID to Name for containers and pods.
//   Used to ensure container and pod IDs are globally unique.
// - nameRegistryBkt: Maps Name to ID for containers and pods.
//   Used to ensure container and pod names are globally unique.
// - nsRegistryBkt: Maps ID to namespace for all containers and pods.
//   Used during lookup operations to determine if a given ID is in the same
//   namespace as the state.
// - ctrBkt: Contains a sub-bucket for each container in the state.
//   Each sub-bucket has config and state keys holding the container's JSON
//   encoded configuration and state (respectively), an optional netNS key
//   containing the path to the container's network namespace, a dependencies
//   bucket containing the container's dependencies, and an optional pod key
//   containing the ID of the pod the container is joined to.
//   After updates to include exec sessions, may also include an exec bucket
//   with the IDs of exec sessions currently in use by the container.
// - allCtrsBkt: Map of ID to name containing only containers. Used for
//   container lookup operations.
// - podBkt: Contains a sub-bucket for each pod in the state.
//   Each sub-bucket has config and state keys holding the pod's JSON encoded
//   configuration and state, plus a containers sub bucket holding the IDs of
//   containers in the pod.
// - allPodsBkt: Map of ID to name containing only pods. Used for pod lookup
//   operations.
// - execBkt: Map of exec session ID to container ID - used for resolving
//   exec session IDs to the containers that hold the exec session.
// - aliasesBkt - Contains a bucket for each CNI network, which contain a map of
//   network alias (an extra name for containers in DNS) to the ID of the
//   container holding the alias. Aliases must be unique per-network, and cannot
//   conflict with names registered in nameRegistryBkt.
// - runtimeConfigBkt: Contains configuration of the libpod instance that
//   initially created the database. This must match for any further instances
//   that access the database, to ensure that state mismatches with
//   containers/storage do not occur.

// NewBoltState creates a new bolt-backed state database
func NewBoltState(path string, runtime *Runtime) (State, error) {
	state := new(BoltState)
	state.dbPath = path
	state.runtime = runtime
	state.namespace = ""
	state.namespaceBytes = nil

	logrus.Debugf("Initializing boltdb state at %s", path)

	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "error opening database %s", path)
	}
	// Everywhere else, we use s.deferredCloseDBCon(db) to ensure the state's DB
	// mutex is also unlocked.
	// However, here, the mutex has not been locked, since we just created
	// the DB connection, and it hasn't left this function yet - no risk of
	// concurrent access.
	// As such, just a db.Close() is fine here.
	defer db.Close()

	createBuckets := [][]byte{
		idRegistryBkt,
		nameRegistryBkt,
		nsRegistryBkt,
		ctrBkt,
		allCtrsBkt,
		podBkt,
		allPodsBkt,
		volBkt,
		allVolsBkt,
		execBkt,
		runtimeConfigBkt,
	}

	// Does the DB need an update?
	needsUpdate := false
	err = db.View(func(tx *bolt.Tx) error {
		for _, bkt := range createBuckets {
			if test := tx.Bucket(bkt); test == nil {
				needsUpdate = true
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "error checking DB schema")
	}

	if !needsUpdate {
		state.valid = true
		return state, nil
	}

	// Ensure schema is properly created in DB
	err = db.Update(func(tx *bolt.Tx) error {
		for _, bkt := range createBuckets {
			if _, err := tx.CreateBucketIfNotExists(bkt); err != nil {
				return errors.Wrapf(err, "error creating bucket %s", string(bkt))
			}
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "error creating buckets for DB")
	}

	state.valid = true

	return state, nil
}

// Close closes the state and prevents further use
func (s *BoltState) Close() error {
	s.valid = false
	return nil
}

// Refresh clears container and pod states after a reboot
func (s *BoltState) Refresh() error {
	if !s.valid {
		return define.ErrDBClosed
	}

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	err = db.Update(func(tx *bolt.Tx) error {
		idBucket, err := getIDBucket(tx)
		if err != nil {
			return err
		}

		ctrsBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		podsBucket, err := getPodBucket(tx)
		if err != nil {
			return err
		}

		allVolsBucket, err := getAllVolsBucket(tx)
		if err != nil {
			return err
		}

		volBucket, err := getVolBucket(tx)
		if err != nil {
			return err
		}

		execBucket, err := getExecBucket(tx)
		if err != nil {
			return err
		}

		// Iterate through all IDs. Check if they are containers.
		// If they are, unmarshal their state, and then clear
		// PID, mountpoint, and state for all of them
		// Then save the modified state
		// Also clear all network namespaces
		err = idBucket.ForEach(func(id, name []byte) error {
			ctrBkt := ctrsBucket.Bucket(id)
			if ctrBkt == nil {
				// It's a pod
				podBkt := podsBucket.Bucket(id)
				if podBkt == nil {
					// This is neither a pod nor a container
					// Error out on the dangling ID
					return errors.Wrapf(define.ErrInternal, "id %s is not a pod or a container", string(id))
				}

				// Get the state
				stateBytes := podBkt.Get(stateKey)
				if stateBytes == nil {
					return errors.Wrapf(define.ErrInternal, "pod %s missing state key", string(id))
				}

				state := new(podState)

				if err := json.Unmarshal(stateBytes, state); err != nil {
					return errors.Wrapf(err, "error unmarshalling state for pod %s", string(id))
				}

				// Clear the CGroup path
				state.CgroupPath = ""

				newStateBytes, err := json.Marshal(state)
				if err != nil {
					return errors.Wrapf(err, "error marshalling modified state for pod %s", string(id))
				}

				if err := podBkt.Put(stateKey, newStateBytes); err != nil {
					return errors.Wrapf(err, "error updating state for pod %s in DB", string(id))
				}

				// It's not a container, nothing to do
				return nil
			}

			// First, delete the network namespace
			if err := ctrBkt.Delete(netNSKey); err != nil {
				return errors.Wrapf(err, "error removing network namespace for container %s", string(id))
			}

			stateBytes := ctrBkt.Get(stateKey)
			if stateBytes == nil {
				// Badly formatted container bucket
				return errors.Wrapf(define.ErrInternal, "container %s missing state in DB", string(id))
			}

			state := new(ContainerState)

			if err := json.Unmarshal(stateBytes, state); err != nil {
				return errors.Wrapf(err, "error unmarshalling state for container %s", string(id))
			}

			resetState(state)

			newStateBytes, err := json.Marshal(state)
			if err != nil {
				return errors.Wrapf(err, "error marshalling modified state for container %s", string(id))
			}

			if err := ctrBkt.Put(stateKey, newStateBytes); err != nil {
				return errors.Wrapf(err, "error updating state for container %s in DB", string(id))
			}

			// Delete all exec sessions, if there are any
			ctrExecBkt := ctrBkt.Bucket(execBkt)
			if ctrExecBkt != nil {
				// Can't delete in a ForEach, so build a list of
				// what to remove then remove.
				toRemove := []string{}
				err = ctrExecBkt.ForEach(func(id, unused []byte) error {
					toRemove = append(toRemove, string(id))
					return nil
				})
				if err != nil {
					return err
				}
				for _, execId := range toRemove {
					if err := ctrExecBkt.Delete([]byte(execId)); err != nil {
						return errors.Wrapf(err, "error removing exec session %s from container %s", execId, string(id))
					}
				}
			}

			return nil
		})
		if err != nil {
			return err
		}

		// Now refresh volumes
		err = allVolsBucket.ForEach(func(id, name []byte) error {
			dbVol := volBucket.Bucket(id)
			if dbVol == nil {
				return errors.Wrapf(define.ErrInternal, "inconsistency in state - volume %s is in all volumes bucket but volume not found", string(id))
			}

			// Get the state
			volStateBytes := dbVol.Get(stateKey)
			if volStateBytes == nil {
				// If the volume doesn't have a state, nothing to do
				return nil
			}

			oldState := new(VolumeState)

			if err := json.Unmarshal(volStateBytes, oldState); err != nil {
				return errors.Wrapf(err, "error unmarshalling state for volume %s", string(id))
			}

			// Reset mount count to 0
			oldState.MountCount = 0

			newState, err := json.Marshal(oldState)
			if err != nil {
				return errors.Wrapf(err, "error marshalling state for volume %s", string(id))
			}

			if err := dbVol.Put(stateKey, newState); err != nil {
				return errors.Wrapf(err, "error storing new state for volume %s", string(id))
			}

			return nil
		})
		if err != nil {
			return err
		}

		// Now refresh exec sessions
		// We want to remove them all, but for-each can't modify buckets
		// So we have to make a list of what to operate on, then do the
		// work.
		toRemoveExec := []string{}
		err = execBucket.ForEach(func(id, unused []byte) error {
			toRemoveExec = append(toRemoveExec, string(id))
			return nil
		})
		if err != nil {
			return err
		}

		for _, execSession := range toRemoveExec {
			if err := execBucket.Delete([]byte(execSession)); err != nil {
				return errors.Wrapf(err, "error deleting exec session %s registry from database", execSession)
			}
		}

		return nil
	})
	return err
}

// GetDBConfig retrieves runtime configuration fields that were created when
// the database was first initialized
func (s *BoltState) GetDBConfig() (*DBConfig, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	cfg := new(DBConfig)

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		configBucket, err := getRuntimeConfigBucket(tx)
		if err != nil {
			return nil
		}

		// Some of these may be nil
		// When we convert to string, Go will coerce them to ""
		// That's probably fine - we could raise an error if the key is
		// missing, but just not including it is also OK.
		libpodRoot := configBucket.Get(staticDirKey)
		libpodTmp := configBucket.Get(tmpDirKey)
		storageRoot := configBucket.Get(graphRootKey)
		storageTmp := configBucket.Get(runRootKey)
		graphDriver := configBucket.Get(graphDriverKey)
		volumePath := configBucket.Get(volPathKey)

		cfg.LibpodRoot = string(libpodRoot)
		cfg.LibpodTmp = string(libpodTmp)
		cfg.StorageRoot = string(storageRoot)
		cfg.StorageTmp = string(storageTmp)
		cfg.GraphDriver = string(graphDriver)
		cfg.VolumePath = string(volumePath)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// ValidateDBConfig validates paths in the given runtime against the database
func (s *BoltState) ValidateDBConfig(runtime *Runtime) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	// Check runtime configuration
	if err := checkRuntimeConfig(db, runtime); err != nil {
		return err
	}

	return nil
}

// SetNamespace sets the namespace that will be used for container and pod
// retrieval
func (s *BoltState) SetNamespace(ns string) error {
	s.namespace = ns

	if ns != "" {
		s.namespaceBytes = []byte(ns)
	} else {
		s.namespaceBytes = nil
	}

	return nil
}

// GetName returns the name associated with a given ID. Since IDs are globally
// unique, it works for both containers and pods.
// Returns ErrNoSuchCtr if the ID does not exist.
func (s *BoltState) GetName(id string) (string, error) {
	if id == "" {
		return "", define.ErrEmptyID
	}

	if !s.valid {
		return "", define.ErrDBClosed
	}

	idBytes := []byte(id)

	db, err := s.getDBCon()
	if err != nil {
		return "", err
	}
	defer s.deferredCloseDBCon(db)

	name := ""

	err = db.View(func(tx *bolt.Tx) error {
		idBkt, err := getIDBucket(tx)
		if err != nil {
			return err
		}

		nameBytes := idBkt.Get(idBytes)
		if nameBytes == nil {
			return define.ErrNoSuchCtr
		}

		if s.namespaceBytes != nil {
			nsBkt, err := getNSBucket(tx)
			if err != nil {
				return err
			}

			idNs := nsBkt.Get(idBytes)
			if !bytes.Equal(idNs, s.namespaceBytes) {
				return define.ErrNoSuchCtr
			}
		}

		name = string(nameBytes)
		return nil
	})
	if err != nil {
		return "", err
	}

	return name, nil
}

// Container retrieves a single container from the state by its full ID
func (s *BoltState) Container(id string) (*Container, error) {
	if id == "" {
		return nil, define.ErrEmptyID
	}

	if !s.valid {
		return nil, define.ErrDBClosed
	}

	ctrID := []byte(id)

	ctr := new(Container)
	ctr.config = new(ContainerConfig)
	ctr.state = new(ContainerState)

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		return s.getContainerFromDB(ctrID, ctr, ctrBucket)
	})
	if err != nil {
		return nil, err
	}

	return ctr, nil
}

// LookupContainerID retrieves a container ID from the state by full or unique
// partial ID or name
func (s *BoltState) LookupContainerID(idOrName string) (string, error) {
	if idOrName == "" {
		return "", define.ErrEmptyID
	}

	if !s.valid {
		return "", define.ErrDBClosed
	}

	db, err := s.getDBCon()
	if err != nil {
		return "", err
	}
	defer s.deferredCloseDBCon(db)

	var id []byte
	err = db.View(func(tx *bolt.Tx) error {
		ctrBucket, err := getCtrBucket(tx)
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

		fullID, err := s.lookupContainerID(idOrName, ctrBucket, namesBucket, nsBucket)
		// Check if it is in our namespace
		if s.namespaceBytes != nil {
			ns := nsBucket.Get(fullID)
			if !bytes.Equal(ns, s.namespaceBytes) {
				return errors.Wrapf(define.ErrNoSuchCtr, "no container found with name or ID %s", idOrName)
			}
		}
		id = fullID
		return err
	})

	if err != nil {
		return "", err
	}

	retID := string(id)
	return retID, nil
}

// LookupContainer retrieves a container from the state by full or unique
// partial ID or name
func (s *BoltState) LookupContainer(idOrName string) (*Container, error) {
	if idOrName == "" {
		return nil, define.ErrEmptyID
	}

	if !s.valid {
		return nil, define.ErrDBClosed
	}

	ctr := new(Container)
	ctr.config = new(ContainerConfig)
	ctr.state = new(ContainerState)

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		ctrBucket, err := getCtrBucket(tx)
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

		id, err := s.lookupContainerID(idOrName, ctrBucket, namesBucket, nsBucket)
		if err != nil {
			return err
		}

		return s.getContainerFromDB(id, ctr, ctrBucket)
	})
	if err != nil {
		return nil, err
	}

	return ctr, nil
}

// HasContainer checks if a container is present in the state
func (s *BoltState) HasContainer(id string) (bool, error) {
	if id == "" {
		return false, define.ErrEmptyID
	}

	if !s.valid {
		return false, define.ErrDBClosed
	}

	ctrID := []byte(id)

	db, err := s.getDBCon()
	if err != nil {
		return false, err
	}
	defer s.deferredCloseDBCon(db)

	exists := false

	err = db.View(func(tx *bolt.Tx) error {
		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		ctrDB := ctrBucket.Bucket(ctrID)
		if ctrDB != nil {
			if s.namespaceBytes != nil {
				nsBytes := ctrDB.Get(namespaceKey)
				if bytes.Equal(nsBytes, s.namespaceBytes) {
					exists = true
				}
			} else {
				exists = true
			}
		}

		return nil
	})
	if err != nil {
		return false, err
	}

	return exists, nil
}

// AddContainer adds a container to the state
// The container being added cannot belong to a pod
func (s *BoltState) AddContainer(ctr *Container) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !ctr.valid {
		return define.ErrCtrRemoved
	}

	if ctr.config.Pod != "" {
		return errors.Wrapf(define.ErrInvalidArg, "cannot add a container that belongs to a pod with AddContainer - use AddContainerToPod")
	}

	return s.addContainer(ctr, nil)
}

// RemoveContainer removes a container from the state
// Only removes containers not in pods - for containers that are a member of a
// pod, use RemoveContainerFromPod
func (s *BoltState) RemoveContainer(ctr *Container) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if ctr.config.Pod != "" {
		return errors.Wrapf(define.ErrPodExists, "container %s is part of a pod, use RemoveContainerFromPod instead", ctr.ID())
	}

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	err = db.Update(func(tx *bolt.Tx) error {
		return s.removeContainer(ctr, nil, tx)
	})
	return err
}

// UpdateContainer updates a container's state from the database
func (s *BoltState) UpdateContainer(ctr *Container) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !ctr.valid {
		return define.ErrCtrRemoved
	}

	if s.namespace != "" && s.namespace != ctr.config.Namespace {
		return errors.Wrapf(define.ErrNSMismatch, "container %s is in namespace %q, does not match our namespace %q", ctr.ID(), ctr.config.Namespace, s.namespace)
	}

	newState := new(ContainerState)
	netNSPath := ""

	ctrID := []byte(ctr.ID())

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		ctrToUpdate := ctrBucket.Bucket(ctrID)
		if ctrToUpdate == nil {
			ctr.valid = false
			return errors.Wrapf(define.ErrNoSuchCtr, "container %s does not exist in database", ctr.ID())
		}

		newStateBytes := ctrToUpdate.Get(stateKey)
		if newStateBytes == nil {
			return errors.Wrapf(define.ErrInternal, "container %s does not have a state key in DB", ctr.ID())
		}

		if err := json.Unmarshal(newStateBytes, newState); err != nil {
			return errors.Wrapf(err, "error unmarshalling container %s state", ctr.ID())
		}

		netNSBytes := ctrToUpdate.Get(netNSKey)
		if netNSBytes != nil {
			netNSPath = string(netNSBytes)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Handle network namespace.
	if os.Geteuid() == 0 {
		// Do it only when root, either on the host or as root in the
		// user namespace.
		if err := replaceNetNS(netNSPath, ctr, newState); err != nil {
			return err
		}
	}

	// New state compiled successfully, swap it into the current state
	ctr.state = newState

	return nil
}

// SaveContainer saves a container's current state in the database
func (s *BoltState) SaveContainer(ctr *Container) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !ctr.valid {
		return define.ErrCtrRemoved
	}

	if s.namespace != "" && s.namespace != ctr.config.Namespace {
		return errors.Wrapf(define.ErrNSMismatch, "container %s is in namespace %q, does not match our namespace %q", ctr.ID(), ctr.config.Namespace, s.namespace)
	}

	stateJSON, err := json.Marshal(ctr.state)
	if err != nil {
		return errors.Wrapf(err, "error marshalling container %s state to JSON", ctr.ID())
	}
	netNSPath := getNetNSPath(ctr)

	ctrID := []byte(ctr.ID())

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	err = db.Update(func(tx *bolt.Tx) error {
		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		ctrToSave := ctrBucket.Bucket(ctrID)
		if ctrToSave == nil {
			ctr.valid = false
			return errors.Wrapf(define.ErrNoSuchCtr, "container %s does not exist in DB", ctr.ID())
		}

		// Update the state
		if err := ctrToSave.Put(stateKey, stateJSON); err != nil {
			return errors.Wrapf(err, "error updating container %s state in DB", ctr.ID())
		}

		if netNSPath != "" {
			if err := ctrToSave.Put(netNSKey, []byte(netNSPath)); err != nil {
				return errors.Wrapf(err, "error updating network namespace path for container %s in DB", ctr.ID())
			}
		} else {
			// Delete the existing network namespace
			if err := ctrToSave.Delete(netNSKey); err != nil {
				return errors.Wrapf(err, "error removing network namespace path for container %s in DB", ctr.ID())
			}
		}

		return nil
	})
	return err
}

// ContainerInUse checks if other containers depend on the given container
// It returns a slice of the IDs of the containers depending on the given
// container. If the slice is empty, no containers depend on the given container
func (s *BoltState) ContainerInUse(ctr *Container) ([]string, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	if !ctr.valid {
		return nil, define.ErrCtrRemoved
	}

	if s.namespace != "" && s.namespace != ctr.config.Namespace {
		return nil, errors.Wrapf(define.ErrNSMismatch, "container %s is in namespace %q, does not match our namespace %q", ctr.ID(), ctr.config.Namespace, s.namespace)
	}

	depCtrs := []string{}

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		ctrDB := ctrBucket.Bucket([]byte(ctr.ID()))
		if ctrDB == nil {
			ctr.valid = false
			return errors.Wrapf(define.ErrNoSuchCtr, "no container with ID %s found in DB", ctr.ID())
		}

		dependsBkt := ctrDB.Bucket(dependenciesBkt)
		if dependsBkt == nil {
			return errors.Wrapf(define.ErrInternal, "container %s has no dependencies bucket", ctr.ID())
		}

		// Iterate through and add dependencies
		err = dependsBkt.ForEach(func(id, value []byte) error {
			depCtrs = append(depCtrs, string(id))

			return nil
		})
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return depCtrs, nil

}

// AllContainers retrieves all the containers in the database
func (s *BoltState) AllContainers() ([]*Container, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	ctrs := []*Container{}

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		allCtrsBucket, err := getAllCtrsBucket(tx)
		if err != nil {
			return err
		}

		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		return allCtrsBucket.ForEach(func(id, name []byte) error {
			// If performance becomes an issue, this check can be
			// removed. But the error messages that come back will
			// be much less helpful.
			ctrExists := ctrBucket.Bucket(id)
			if ctrExists == nil {
				return errors.Wrapf(define.ErrInternal, "state is inconsistent - container ID %s in all containers, but container not found", string(id))
			}

			ctr := new(Container)
			ctr.config = new(ContainerConfig)
			ctr.state = new(ContainerState)

			if err := s.getContainerFromDB(id, ctr, ctrBucket); err != nil {
				// If the error is a namespace mismatch, we can
				// ignore it safely.
				// We just won't include the container in the
				// results.
				if errors.Cause(err) != define.ErrNSMismatch {
					// Even if it's not an NS mismatch, it's
					// not worth erroring over.
					// If we do, a single bad container JSON
					// could render libpod unusable.
					logrus.Errorf("Error retrieving container %s from the database: %v", string(id), err)
				}
			} else {
				ctrs = append(ctrs, ctr)
			}

			return nil

		})
	})
	if err != nil {
		return nil, err
	}

	return ctrs, nil
}

// GetNetworks returns the CNI networks this container is a part of.
func (s *BoltState) GetNetworks(ctr *Container) ([]string, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	if !ctr.valid {
		return nil, define.ErrCtrRemoved
	}

	if s.namespace != "" && s.namespace != ctr.config.Namespace {
		return nil, errors.Wrapf(define.ErrNSMismatch, "container %s is in namespace %q, does not match our namespace %q", ctr.ID(), ctr.config.Namespace, s.namespace)
	}

	ctrID := []byte(ctr.ID())

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.deferredCloseDBCon(db)

	networks := []string{}

	err = db.View(func(tx *bolt.Tx) error {
		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		dbCtr := ctrBucket.Bucket(ctrID)
		if dbCtr == nil {
			ctr.valid = false
			return errors.Wrapf(define.ErrNoSuchCtr, "container %s does not exist in database", ctr.ID())
		}

		ctrNetworkBkt := dbCtr.Bucket(networksBkt)
		if ctrNetworkBkt == nil {
			return errors.Wrapf(define.ErrNoSuchNetwork, "container %s is not joined to any CNI networks", ctr.ID())
		}

		return ctrNetworkBkt.ForEach(func(network, v []byte) error {
			networks = append(networks, string(network))
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return networks, nil
}

// GetNetworkAliases retrieves the network aliases for the given container in
// the given CNI network.
func (s *BoltState) GetNetworkAliases(ctr *Container, network string) ([]string, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	if !ctr.valid {
		return nil, define.ErrCtrRemoved
	}

	if network == "" {
		return nil, errors.Wrapf(define.ErrInvalidArg, "network names must not be empty")
	}

	if s.namespace != "" && s.namespace != ctr.config.Namespace {
		return nil, errors.Wrapf(define.ErrNSMismatch, "container %s is in namespace %q, does not match our namespace %q", ctr.ID(), ctr.config.Namespace, s.namespace)
	}

	ctrID := []byte(ctr.ID())

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.deferredCloseDBCon(db)

	aliases := []string{}

	err = db.View(func(tx *bolt.Tx) error {
		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		dbCtr := ctrBucket.Bucket(ctrID)
		if dbCtr == nil {
			ctr.valid = false
			return errors.Wrapf(define.ErrNoSuchCtr, "container %s does not exist in database", ctr.ID())
		}

		ctrNetworkBkt := dbCtr.Bucket(networksBkt)
		if ctrNetworkBkt == nil {
			// No networks joined, so no aliases
			return nil
		}

		inNetwork := ctrNetworkBkt.Get([]byte(network))
		if inNetwork == nil {
			return errors.Wrapf(define.ErrNoAliases, "container %s is not part of network %s, no aliases found", ctr.ID(), network)
		}

		ctrAliasesBkt := dbCtr.Bucket(aliasesBkt)
		if ctrAliasesBkt == nil {
			// No aliases
			return nil
		}

		netAliasesBkt := ctrAliasesBkt.Bucket([]byte(network))
		if netAliasesBkt == nil {
			// No aliases for this specific network.
			return nil
		}

		return netAliasesBkt.ForEach(func(alias, v []byte) error {
			aliases = append(aliases, string(alias))
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	return aliases, nil
}

// GetAllNetworkAliases retrieves the network aliases for the given container in
// all CNI networks.
func (s *BoltState) GetAllNetworkAliases(ctr *Container) (map[string][]string, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	if !ctr.valid {
		return nil, define.ErrCtrRemoved
	}

	if s.namespace != "" && s.namespace != ctr.config.Namespace {
		return nil, errors.Wrapf(define.ErrNSMismatch, "container %s is in namespace %q, does not match our namespace %q", ctr.ID(), ctr.config.Namespace, s.namespace)
	}

	ctrID := []byte(ctr.ID())

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.deferredCloseDBCon(db)

	aliases := make(map[string][]string)

	err = db.View(func(tx *bolt.Tx) error {
		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		dbCtr := ctrBucket.Bucket(ctrID)
		if dbCtr == nil {
			ctr.valid = false
			return errors.Wrapf(define.ErrNoSuchCtr, "container %s does not exist in database", ctr.ID())
		}

		ctrAliasesBkt := dbCtr.Bucket(aliasesBkt)
		if ctrAliasesBkt == nil {
			// No aliases present
			return nil
		}

		ctrNetworkBkt := dbCtr.Bucket(networksBkt)
		if ctrNetworkBkt == nil {
			// No networks joined, so no aliases
			return nil
		}

		return ctrNetworkBkt.ForEach(func(network, v []byte) error {
			netAliasesBkt := ctrAliasesBkt.Bucket(network)
			if netAliasesBkt == nil {
				return nil
			}

			netAliases := []string{}

			_ = netAliasesBkt.ForEach(func(alias, v []byte) error {
				netAliases = append(netAliases, string(alias))
				return nil
			})

			aliases[string(network)] = netAliases
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	return aliases, nil
}

// NetworkConnect adds the given container to the given network. If aliases are
// specified, those will be added to the given network.
func (s *BoltState) NetworkConnect(ctr *Container, network string, aliases []string) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !ctr.valid {
		return define.ErrCtrRemoved
	}

	if network == "" {
		return errors.Wrapf(define.ErrInvalidArg, "network names must not be empty")
	}

	if s.namespace != "" && s.namespace != ctr.config.Namespace {
		return errors.Wrapf(define.ErrNSMismatch, "container %s is in namespace %q, does not match our namespace %q", ctr.ID(), ctr.config.Namespace, s.namespace)
	}

	ctrID := []byte(ctr.ID())

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	return db.Update(func(tx *bolt.Tx) error {
		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		dbCtr := ctrBucket.Bucket(ctrID)
		if dbCtr == nil {
			ctr.valid = false
			return errors.Wrapf(define.ErrNoSuchCtr, "container %s does not exist in database", ctr.ID())
		}

		ctrAliasesBkt, err := dbCtr.CreateBucketIfNotExists(aliasesBkt)
		if err != nil {
			return errors.Wrapf(err, "error creating aliases bucket for container %s", ctr.ID())
		}

		ctrNetworksBkt := dbCtr.Bucket(networksBkt)
		if ctrNetworksBkt == nil {
			ctrNetworksBkt, err = dbCtr.CreateBucket(networksBkt)
			if err != nil {
				return errors.Wrapf(err, "error creating networks bucket for container %s", ctr.ID())
			}
			ctrNetworks := ctr.config.Networks
			if len(ctrNetworks) == 0 {
				ctrNetworks = []string{ctr.runtime.netPlugin.GetDefaultNetworkName()}
			}
			// Copy in all the container's CNI networks
			for _, net := range ctrNetworks {
				if err := ctrNetworksBkt.Put([]byte(net), ctrID); err != nil {
					return errors.Wrapf(err, "error adding container %s network %s to DB", ctr.ID(), net)
				}
			}
		}
		netConnected := ctrNetworksBkt.Get([]byte(network))
		if netConnected != nil {
			return errors.Wrapf(define.ErrNetworkExists, "container %s is already connected to CNI network %q", ctr.ID(), network)
		}

		// Add the network
		if err := ctrNetworksBkt.Put([]byte(network), ctrID); err != nil {
			return errors.Wrapf(err, "error adding container %s to network %s in DB", ctr.ID(), network)
		}

		ctrNetAliasesBkt, err := ctrAliasesBkt.CreateBucketIfNotExists([]byte(network))
		if err != nil {
			return errors.Wrapf(err, "error adding container %s network aliases bucket for network %s", ctr.ID(), network)
		}
		for _, alias := range aliases {
			if err := ctrNetAliasesBkt.Put([]byte(alias), ctrID); err != nil {
				return errors.Wrapf(err, "error adding container %s network alias %s for network %s", ctr.ID(), alias, network)
			}
		}
		return nil
	})
}

// NetworkDisconnect disconnects the container from the given network, also
// removing any aliases in the network.
func (s *BoltState) NetworkDisconnect(ctr *Container, network string) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !ctr.valid {
		return define.ErrCtrRemoved
	}

	if network == "" {
		return errors.Wrapf(define.ErrInvalidArg, "network names must not be empty")
	}

	if s.namespace != "" && s.namespace != ctr.config.Namespace {
		return errors.Wrapf(define.ErrNSMismatch, "container %s is in namespace %q, does not match our namespace %q", ctr.ID(), ctr.config.Namespace, s.namespace)
	}

	ctrID := []byte(ctr.ID())

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	return db.Update(func(tx *bolt.Tx) error {
		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		dbCtr := ctrBucket.Bucket(ctrID)
		if dbCtr == nil {
			ctr.valid = false
			return errors.Wrapf(define.ErrNoSuchCtr, "container %s does not exist in database", ctr.ID())
		}

		ctrAliasesBkt := dbCtr.Bucket(aliasesBkt)
		ctrNetworksBkt := dbCtr.Bucket(networksBkt)
		if ctrNetworksBkt == nil {
			return errors.Wrapf(define.ErrNoSuchNetwork, "container %s is not connected to any CNI networks, so cannot disconnect", ctr.ID())
		}
		netConnected := ctrNetworksBkt.Get([]byte(network))
		if netConnected == nil {
			return errors.Wrapf(define.ErrNoSuchNetwork, "container %s is not connected to CNI network %q", ctr.ID(), network)
		}

		if err := ctrNetworksBkt.Delete([]byte(network)); err != nil {
			return errors.Wrapf(err, "error removing container %s from network %s", ctr.ID(), network)
		}

		if ctrAliasesBkt != nil {
			bktExists := ctrAliasesBkt.Bucket([]byte(network))
			if bktExists == nil {
				return nil
			}

			if err := ctrAliasesBkt.DeleteBucket([]byte(network)); err != nil {
				return errors.Wrapf(err, "error removing container %s network aliases for network %s", ctr.ID(), network)
			}
		}

		return nil
	})
}

// GetContainerConfig returns a container config from the database by full ID
func (s *BoltState) GetContainerConfig(id string) (*ContainerConfig, error) {
	if len(id) == 0 {
		return nil, define.ErrEmptyID
	}

	if !s.valid {
		return nil, define.ErrDBClosed
	}

	config := new(ContainerConfig)

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		return s.getContainerConfigFromDB([]byte(id), config, ctrBucket)
	})
	if err != nil {
		return nil, err
	}

	return config, nil
}

// AddExecSession adds an exec session to the state.
func (s *BoltState) AddExecSession(ctr *Container, session *ExecSession) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !ctr.valid {
		return define.ErrCtrRemoved
	}

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	ctrID := []byte(ctr.ID())
	sessionID := []byte(session.ID())

	err = db.Update(func(tx *bolt.Tx) error {
		execBucket, err := getExecBucket(tx)
		if err != nil {
			return err
		}
		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		dbCtr := ctrBucket.Bucket(ctrID)
		if dbCtr == nil {
			ctr.valid = false
			return errors.Wrapf(define.ErrNoSuchCtr, "container %s is not present in the database", ctr.ID())
		}

		ctrExecSessionBucket, err := dbCtr.CreateBucketIfNotExists(execBkt)
		if err != nil {
			return errors.Wrapf(err, "error creating exec sessions bucket for container %s", ctr.ID())
		}

		execExists := execBucket.Get(sessionID)
		if execExists != nil {
			return errors.Wrapf(define.ErrExecSessionExists, "an exec session with ID %s already exists", session.ID())
		}

		if err := execBucket.Put(sessionID, ctrID); err != nil {
			return errors.Wrapf(err, "error adding exec session %s to DB", session.ID())
		}

		if err := ctrExecSessionBucket.Put(sessionID, ctrID); err != nil {
			return errors.Wrapf(err, "error adding exec session %s to container %s in DB", session.ID(), ctr.ID())
		}

		return nil
	})
	return err
}

// GetExecSession returns the ID of the container an exec session is associated
// with.
func (s *BoltState) GetExecSession(id string) (string, error) {
	if !s.valid {
		return "", define.ErrDBClosed
	}

	if id == "" {
		return "", define.ErrEmptyID
	}

	db, err := s.getDBCon()
	if err != nil {
		return "", err
	}
	defer s.deferredCloseDBCon(db)

	ctrID := ""
	err = db.View(func(tx *bolt.Tx) error {
		execBucket, err := getExecBucket(tx)
		if err != nil {
			return err
		}

		ctr := execBucket.Get([]byte(id))
		if ctr == nil {
			return errors.Wrapf(define.ErrNoSuchExecSession, "no exec session with ID %s found", id)
		}
		ctrID = string(ctr)
		return nil
	})
	return ctrID, err
}

// RemoveExecSession removes references to the given exec session in the
// database.
func (s *BoltState) RemoveExecSession(session *ExecSession) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	sessionID := []byte(session.ID())
	containerID := []byte(session.ContainerID())
	err = db.Update(func(tx *bolt.Tx) error {
		execBucket, err := getExecBucket(tx)
		if err != nil {
			return err
		}
		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		sessionExists := execBucket.Get(sessionID)
		if sessionExists == nil {
			return define.ErrNoSuchExecSession
		}
		// Check that container ID matches
		if string(sessionExists) != session.ContainerID() {
			return errors.Wrapf(define.ErrInternal, "database inconsistency: exec session %s points to container %s in state but %s in database", session.ID(), session.ContainerID(), string(sessionExists))
		}

		if err := execBucket.Delete(sessionID); err != nil {
			return errors.Wrapf(err, "error removing exec session %s from database", session.ID())
		}

		dbCtr := ctrBucket.Bucket(containerID)
		if dbCtr == nil {
			// State is inconsistent. We refer to a container that
			// is no longer in the state.
			// Return without error, to attempt to recover.
			return nil
		}

		ctrExecBucket := dbCtr.Bucket(execBkt)
		if ctrExecBucket == nil {
			// Again, state is inconsistent. We should have an exec
			// bucket, and it should have this session.
			// Again, nothing we can do, so proceed and try to
			// recover.
			return nil
		}

		ctrSessionExists := ctrExecBucket.Get(sessionID)
		if ctrSessionExists != nil {
			if err := ctrExecBucket.Delete(sessionID); err != nil {
				return errors.Wrapf(err, "error removing exec session %s from container %s in database", session.ID(), session.ContainerID())
			}
		}

		return nil
	})
	return err
}

// GetContainerExecSessions retrieves the IDs of all exec sessions running in a
// container that the database is aware of (IE, were added via AddExecSession).
func (s *BoltState) GetContainerExecSessions(ctr *Container) ([]string, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	if !ctr.valid {
		return nil, define.ErrCtrRemoved
	}

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.deferredCloseDBCon(db)

	ctrID := []byte(ctr.ID())
	sessions := []string{}
	err = db.View(func(tx *bolt.Tx) error {
		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		dbCtr := ctrBucket.Bucket(ctrID)
		if dbCtr == nil {
			ctr.valid = false
			return define.ErrNoSuchCtr
		}

		ctrExecSessions := dbCtr.Bucket(execBkt)
		if ctrExecSessions == nil {
			return nil
		}

		return ctrExecSessions.ForEach(func(id, unused []byte) error {
			sessions = append(sessions, string(id))
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	return sessions, nil
}

// RemoveContainerExecSessions removes all exec sessions attached to a given
// container.
func (s *BoltState) RemoveContainerExecSessions(ctr *Container) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !ctr.valid {
		return define.ErrCtrRemoved
	}

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	ctrID := []byte(ctr.ID())
	sessions := []string{}

	err = db.Update(func(tx *bolt.Tx) error {
		execBucket, err := getExecBucket(tx)
		if err != nil {
			return err
		}
		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		dbCtr := ctrBucket.Bucket(ctrID)
		if dbCtr == nil {
			ctr.valid = false
			return define.ErrNoSuchCtr
		}

		ctrExecSessions := dbCtr.Bucket(execBkt)
		if ctrExecSessions == nil {
			return nil
		}

		err = ctrExecSessions.ForEach(func(id, unused []byte) error {
			sessions = append(sessions, string(id))
			return nil
		})
		if err != nil {
			return err
		}

		for _, session := range sessions {
			if err := ctrExecSessions.Delete([]byte(session)); err != nil {
				return errors.Wrapf(err, "error removing container %s exec session %s from database", ctr.ID(), session)
			}
			// Check if the session exists in the global table
			// before removing. It should, but in cases where the DB
			// has become inconsistent, we should try and proceed
			// so we can recover.
			sessionExists := execBucket.Get([]byte(session))
			if sessionExists == nil {
				continue
			}
			if string(sessionExists) != ctr.ID() {
				return errors.Wrapf(define.ErrInternal, "database mismatch: exec session %s is associated with containers %s and %s", session, ctr.ID(), string(sessionExists))
			}
			if err := execBucket.Delete([]byte(session)); err != nil {
				return errors.Wrapf(err, "error removing container %s exec session %s from exec sessions", ctr.ID(), session)
			}
		}

		return nil
	})
	return err
}

// RewriteContainerConfig rewrites a container's configuration.
// WARNING: This function is DANGEROUS. Do not use without reading the full
// comment on this function in state.go.
func (s *BoltState) RewriteContainerConfig(ctr *Container, newCfg *ContainerConfig) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !ctr.valid {
		return define.ErrCtrRemoved
	}

	newCfgJSON, err := json.Marshal(newCfg)
	if err != nil {
		return errors.Wrapf(err, "error marshalling new configuration JSON for container %s", ctr.ID())
	}

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	err = db.Update(func(tx *bolt.Tx) error {
		ctrBkt, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		ctrDB := ctrBkt.Bucket([]byte(ctr.ID()))
		if ctrDB == nil {
			ctr.valid = false
			return errors.Wrapf(define.ErrNoSuchCtr, "no container with ID %s found in DB", ctr.ID())
		}

		if err := ctrDB.Put(configKey, newCfgJSON); err != nil {
			return errors.Wrapf(err, "error updating container %s config JSON", ctr.ID())
		}

		return nil
	})
	return err
}

// RewritePodConfig rewrites a pod's configuration.
// WARNING: This function is DANGEROUS. Do not use without reading the full
// comment on this function in state.go.
func (s *BoltState) RewritePodConfig(pod *Pod, newCfg *PodConfig) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !pod.valid {
		return define.ErrPodRemoved
	}

	newCfgJSON, err := json.Marshal(newCfg)
	if err != nil {
		return errors.Wrapf(err, "error marshalling new configuration JSON for pod %s", pod.ID())
	}

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	err = db.Update(func(tx *bolt.Tx) error {
		podBkt, err := getPodBucket(tx)
		if err != nil {
			return err
		}

		podDB := podBkt.Bucket([]byte(pod.ID()))
		if podDB == nil {
			pod.valid = false
			return errors.Wrapf(define.ErrNoSuchPod, "no pod with ID %s found in DB", pod.ID())
		}

		if err := podDB.Put(configKey, newCfgJSON); err != nil {
			return errors.Wrapf(err, "error updating pod %s config JSON", pod.ID())
		}

		return nil
	})
	return err
}

// RewriteVolumeConfig rewrites a volume's configuration.
// WARNING: This function is DANGEROUS. Do not use without reading the full
// comment on this function in state.go.
func (s *BoltState) RewriteVolumeConfig(volume *Volume, newCfg *VolumeConfig) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !volume.valid {
		return define.ErrVolumeRemoved
	}

	newCfgJSON, err := json.Marshal(newCfg)
	if err != nil {
		return errors.Wrapf(err, "error marshalling new configuration JSON for volume %q", volume.Name())
	}

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	err = db.Update(func(tx *bolt.Tx) error {
		volBkt, err := getVolBucket(tx)
		if err != nil {
			return err
		}

		volDB := volBkt.Bucket([]byte(volume.Name()))
		if volDB == nil {
			volume.valid = false
			return errors.Wrapf(define.ErrNoSuchVolume, "no volume with name %q found in DB", volume.Name())
		}

		if err := volDB.Put(configKey, newCfgJSON); err != nil {
			return errors.Wrapf(err, "error updating volume %q config JSON", volume.Name())
		}

		return nil
	})
	return err
}

// Pod retrieves a pod given its full ID
func (s *BoltState) Pod(id string) (*Pod, error) {
	if id == "" {
		return nil, define.ErrEmptyID
	}

	if !s.valid {
		return nil, define.ErrDBClosed
	}

	podID := []byte(id)

	pod := new(Pod)
	pod.config = new(PodConfig)
	pod.state = new(podState)

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		podBkt, err := getPodBucket(tx)
		if err != nil {
			return err
		}

		return s.getPodFromDB(podID, pod, podBkt)
	})
	if err != nil {
		return nil, err
	}

	return pod, nil
}

// LookupPod retrieves a pod from full or unique partial ID or name
func (s *BoltState) LookupPod(idOrName string) (*Pod, error) {
	if idOrName == "" {
		return nil, define.ErrEmptyID
	}

	if !s.valid {
		return nil, define.ErrDBClosed
	}

	pod := new(Pod)
	pod.config = new(PodConfig)
	pod.state = new(podState)

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		podBkt, err := getPodBucket(tx)
		if err != nil {
			return err
		}

		namesBkt, err := getNamesBucket(tx)
		if err != nil {
			return err
		}

		nsBkt, err := getNSBucket(tx)
		if err != nil {
			return err
		}

		// First, check if the ID given was the actual pod ID
		var id []byte
		podExists := podBkt.Bucket([]byte(idOrName))
		if podExists != nil {
			// A full pod ID was given.
			// It might not be in our namespace, but getPodFromDB()
			// will handle that case.
			id = []byte(idOrName)
			return s.getPodFromDB(id, pod, podBkt)
		}

		// Next, check if the full name was given
		isCtr := false
		fullID := namesBkt.Get([]byte(idOrName))
		if fullID != nil {
			// The name exists and maps to an ID.
			// However, we aren't yet sure if the ID is a pod.
			podExists = podBkt.Bucket(fullID)
			if podExists != nil {
				// A pod bucket matching the full ID was found.
				return s.getPodFromDB(fullID, pod, podBkt)
			}
			// Don't error if we have a name match but it's not a
			// pod - there's a chance we have a pod with an ID
			// starting with those characters.
			// However, so we can return a good error, note whether
			// this is a container.
			isCtr = true
		}
		// They did not give us a full pod name or ID.
		// Search for partial ID matches.
		exists := false
		err = podBkt.ForEach(func(checkID, checkName []byte) error {
			// If the pod isn't in our namespace, we
			// can't match it
			if s.namespaceBytes != nil {
				ns := nsBkt.Get(checkID)
				if !bytes.Equal(ns, s.namespaceBytes) {
					return nil
				}
			}
			if strings.HasPrefix(string(checkID), idOrName) {
				if exists {
					return errors.Wrapf(define.ErrPodExists, "more than one result for ID or name %s", idOrName)
				}
				id = checkID
				exists = true
			}

			return nil
		})
		if err != nil {
			return err
		} else if !exists {
			if isCtr {
				return errors.Wrapf(define.ErrNoSuchPod, "%s is a container, not a pod", idOrName)
			}
			return errors.Wrapf(define.ErrNoSuchPod, "no pod with name or ID %s found", idOrName)
		}

		// We might have found a container ID, but it's OK
		// We'll just fail in getPodFromDB with ErrNoSuchPod
		return s.getPodFromDB(id, pod, podBkt)
	})
	if err != nil {
		return nil, err
	}

	return pod, nil
}

// HasPod checks if a pod with the given ID exists in the state
func (s *BoltState) HasPod(id string) (bool, error) {
	if id == "" {
		return false, define.ErrEmptyID
	}

	if !s.valid {
		return false, define.ErrDBClosed
	}

	podID := []byte(id)

	exists := false

	db, err := s.getDBCon()
	if err != nil {
		return false, err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		podBkt, err := getPodBucket(tx)
		if err != nil {
			return err
		}

		podDB := podBkt.Bucket(podID)
		if podDB != nil {
			if s.namespaceBytes != nil {
				podNS := podDB.Get(namespaceKey)
				if bytes.Equal(s.namespaceBytes, podNS) {
					exists = true
				}
			} else {
				exists = true
			}
		}

		return nil
	})
	if err != nil {
		return false, err
	}

	return exists, nil
}

// PodHasContainer checks if the given pod has a container with the given ID
func (s *BoltState) PodHasContainer(pod *Pod, id string) (bool, error) {
	if id == "" {
		return false, define.ErrEmptyID
	}

	if !s.valid {
		return false, define.ErrDBClosed
	}

	if !pod.valid {
		return false, define.ErrPodRemoved
	}

	if s.namespace != "" && s.namespace != pod.config.Namespace {
		return false, errors.Wrapf(define.ErrNSMismatch, "pod %s is in namespace %q but we are in namespace %q", pod.ID(), pod.config.Namespace, s.namespace)
	}

	ctrID := []byte(id)
	podID := []byte(pod.ID())

	exists := false

	db, err := s.getDBCon()
	if err != nil {
		return false, err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		podBkt, err := getPodBucket(tx)
		if err != nil {
			return err
		}

		// Get pod itself
		podDB := podBkt.Bucket(podID)
		if podDB == nil {
			pod.valid = false
			return errors.Wrapf(define.ErrNoSuchPod, "pod %s not found in database", pod.ID())
		}

		// Get pod containers bucket
		podCtrs := podDB.Bucket(containersBkt)
		if podCtrs == nil {
			return errors.Wrapf(define.ErrInternal, "pod %s missing containers bucket in DB", pod.ID())
		}

		// Don't bother with a namespace check on the container -
		// We maintain the invariant that container namespaces must
		// match the namespace of the pod they join.
		// We already checked the pod namespace, so we should be fine.

		ctr := podCtrs.Get(ctrID)
		if ctr != nil {
			exists = true
		}

		return nil
	})
	if err != nil {
		return false, err
	}

	return exists, nil
}

// PodContainersByID returns the IDs of all containers present in the given pod
func (s *BoltState) PodContainersByID(pod *Pod) ([]string, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	if !pod.valid {
		return nil, define.ErrPodRemoved
	}

	if s.namespace != "" && s.namespace != pod.config.Namespace {
		return nil, errors.Wrapf(define.ErrNSMismatch, "pod %s is in namespace %q but we are in namespace %q", pod.ID(), pod.config.Namespace, s.namespace)
	}

	podID := []byte(pod.ID())

	ctrs := []string{}

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		podBkt, err := getPodBucket(tx)
		if err != nil {
			return err
		}

		// Get pod itself
		podDB := podBkt.Bucket(podID)
		if podDB == nil {
			pod.valid = false
			return errors.Wrapf(define.ErrNoSuchPod, "pod %s not found in database", pod.ID())
		}

		// Get pod containers bucket
		podCtrs := podDB.Bucket(containersBkt)
		if podCtrs == nil {
			return errors.Wrapf(define.ErrInternal, "pod %s missing containers bucket in DB", pod.ID())
		}

		// Iterate through all containers in the pod
		err = podCtrs.ForEach(func(id, val []byte) error {
			ctrs = append(ctrs, string(id))

			return nil
		})
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return ctrs, nil
}

// PodContainers returns all the containers present in the given pod
func (s *BoltState) PodContainers(pod *Pod) ([]*Container, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	if !pod.valid {
		return nil, define.ErrPodRemoved
	}

	if s.namespace != "" && s.namespace != pod.config.Namespace {
		return nil, errors.Wrapf(define.ErrNSMismatch, "pod %s is in namespace %q but we are in namespace %q", pod.ID(), pod.config.Namespace, s.namespace)
	}

	podID := []byte(pod.ID())

	ctrs := []*Container{}

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		podBkt, err := getPodBucket(tx)
		if err != nil {
			return err
		}

		ctrBkt, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		// Get pod itself
		podDB := podBkt.Bucket(podID)
		if podDB == nil {
			pod.valid = false
			return errors.Wrapf(define.ErrNoSuchPod, "pod %s not found in database", pod.ID())
		}

		// Get pod containers bucket
		podCtrs := podDB.Bucket(containersBkt)
		if podCtrs == nil {
			return errors.Wrapf(define.ErrInternal, "pod %s missing containers bucket in DB", pod.ID())
		}

		// Iterate through all containers in the pod
		err = podCtrs.ForEach(func(id, val []byte) error {
			newCtr := new(Container)
			newCtr.config = new(ContainerConfig)
			newCtr.state = new(ContainerState)
			ctrs = append(ctrs, newCtr)

			return s.getContainerFromDB(id, newCtr, ctrBkt)
		})
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return ctrs, nil
}

// AddVolume adds the given volume to the state. It also adds ctrDepID to
// the sub bucket holding the container dependencies that this volume has
func (s *BoltState) AddVolume(volume *Volume) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !volume.valid {
		return define.ErrVolumeRemoved
	}

	volName := []byte(volume.Name())

	volConfigJSON, err := json.Marshal(volume.config)
	if err != nil {
		return errors.Wrapf(err, "error marshalling volume %s config to JSON", volume.Name())
	}

	// Volume state is allowed to not exist
	var volStateJSON []byte
	if volume.state != nil {
		volStateJSON, err = json.Marshal(volume.state)
		if err != nil {
			return errors.Wrapf(err, "error marshalling volume %s state to JSON", volume.Name())
		}
	}

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	err = db.Update(func(tx *bolt.Tx) error {
		volBkt, err := getVolBucket(tx)
		if err != nil {
			return err
		}

		allVolsBkt, err := getAllVolsBucket(tx)
		if err != nil {
			return err
		}

		// Check if we already have a volume with the given name
		volExists := allVolsBkt.Get(volName)
		if volExists != nil {
			return errors.Wrapf(define.ErrVolumeExists, "name %s is in use", volume.Name())
		}

		// We are good to add the volume
		// Make a bucket for it
		newVol, err := volBkt.CreateBucket(volName)
		if err != nil {
			return errors.Wrapf(err, "error creating bucket for volume %s", volume.Name())
		}

		// Make a subbucket for the containers using the volume. Dependent container IDs will be addedremoved to
		// this bucket in addcontainer/removeContainer
		if _, err := newVol.CreateBucket(volDependenciesBkt); err != nil {
			return errors.Wrapf(err, "error creating bucket for containers using volume %s", volume.Name())
		}

		if err := newVol.Put(configKey, volConfigJSON); err != nil {
			return errors.Wrapf(err, "error storing volume %s configuration in DB", volume.Name())
		}

		if volStateJSON != nil {
			if err := newVol.Put(stateKey, volStateJSON); err != nil {
				return errors.Wrapf(err, "error storing volume %s state in DB", volume.Name())
			}
		}

		if err := allVolsBkt.Put(volName, volName); err != nil {
			return errors.Wrapf(err, "error storing volume %s in all volumes bucket in DB", volume.Name())
		}

		return nil
	})
	return err
}

// RemoveVolume removes the given volume from the state
func (s *BoltState) RemoveVolume(volume *Volume) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	volName := []byte(volume.Name())

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	err = db.Update(func(tx *bolt.Tx) error {
		volBkt, err := getVolBucket(tx)
		if err != nil {
			return err
		}

		allVolsBkt, err := getAllVolsBucket(tx)
		if err != nil {
			return err
		}

		ctrBkt, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		// Check if the volume exists
		volDB := volBkt.Bucket(volName)
		if volDB == nil {
			volume.valid = false
			return errors.Wrapf(define.ErrNoSuchVolume, "volume %s does not exist in DB", volume.Name())
		}

		// Check if volume is not being used by any container
		// This should never be nil
		// But if it is, we can assume that no containers are using
		// the volume.
		volCtrsBkt := volDB.Bucket(volDependenciesBkt)
		if volCtrsBkt != nil {
			var deps []string
			err = volCtrsBkt.ForEach(func(id, value []byte) error {
				// Alright, this is ugly.
				// But we need it to work around the change in
				// volume dependency handling, to make sure that
				// older Podman versions don't cause DB
				// corruption.
				// Look up all dependencies and see that they
				// still exist before appending.
				ctrExists := ctrBkt.Bucket(id)
				if ctrExists == nil {
					return nil
				}

				deps = append(deps, string(id))
				return nil
			})
			if err != nil {
				return errors.Wrapf(err, "error getting list of dependencies from dependencies bucket for volumes %q", volume.Name())
			}
			if len(deps) > 0 {
				return errors.Wrapf(define.ErrVolumeBeingUsed, "volume %s is being used by container(s) %s", volume.Name(), strings.Join(deps, ","))
			}
		}

		// volume is ready for removal
		// Let's kick it out
		if err := allVolsBkt.Delete(volName); err != nil {
			return errors.Wrapf(err, "error removing volume %s from all volumes bucket in DB", volume.Name())
		}
		if err := volBkt.DeleteBucket(volName); err != nil {
			return errors.Wrapf(err, "error removing volume %s from DB", volume.Name())
		}

		return nil
	})
	return err
}

// UpdateVolume updates the volume's state from the database.
func (s *BoltState) UpdateVolume(volume *Volume) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !volume.valid {
		return define.ErrVolumeRemoved
	}

	newState := new(VolumeState)
	volumeName := []byte(volume.Name())

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		volBucket, err := getVolBucket(tx)
		if err != nil {
			return err
		}

		volToUpdate := volBucket.Bucket(volumeName)
		if volToUpdate == nil {
			volume.valid = false
			return errors.Wrapf(define.ErrNoSuchVolume, "no volume with name %s found in database", volume.Name())
		}

		stateBytes := volToUpdate.Get(stateKey)
		if stateBytes == nil {
			// Having no state is valid.
			// Return nil, use the empty state.
			return nil
		}

		if err := json.Unmarshal(stateBytes, newState); err != nil {
			return errors.Wrapf(err, "error unmarshalling volume %s state", volume.Name())
		}

		return nil
	})
	if err != nil {
		return err
	}

	volume.state = newState

	return nil
}

// SaveVolume saves the volume's state to the database.
func (s *BoltState) SaveVolume(volume *Volume) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !volume.valid {
		return define.ErrVolumeRemoved
	}

	volumeName := []byte(volume.Name())

	var newStateJSON []byte
	if volume.state != nil {
		stateJSON, err := json.Marshal(volume.state)
		if err != nil {
			return errors.Wrapf(err, "error marshalling volume %s state to JSON", volume.Name())
		}
		newStateJSON = stateJSON
	}

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	err = db.Update(func(tx *bolt.Tx) error {
		volBucket, err := getVolBucket(tx)
		if err != nil {
			return err
		}

		volToUpdate := volBucket.Bucket(volumeName)
		if volToUpdate == nil {
			volume.valid = false
			return errors.Wrapf(define.ErrNoSuchVolume, "no volume with name %s found in database", volume.Name())
		}

		return volToUpdate.Put(stateKey, newStateJSON)
	})
	return err
}

// AllVolumes returns all volumes present in the state
func (s *BoltState) AllVolumes() ([]*Volume, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	volumes := []*Volume{}

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		allVolsBucket, err := getAllVolsBucket(tx)
		if err != nil {
			return err
		}

		volBucket, err := getVolBucket(tx)
		if err != nil {
			return err
		}
		err = allVolsBucket.ForEach(func(id, name []byte) error {
			volExists := volBucket.Bucket(id)
			// This check can be removed if performance becomes an
			// issue, but much less helpful errors will be produced
			if volExists == nil {
				return errors.Wrapf(define.ErrInternal, "inconsistency in state - volume %s is in all volumes bucket but volume not found", string(id))
			}

			volume := new(Volume)
			volume.config = new(VolumeConfig)
			volume.state = new(VolumeState)

			if err := s.getVolumeFromDB(id, volume, volBucket); err != nil {
				if errors.Cause(err) != define.ErrNSMismatch {
					logrus.Errorf("Error retrieving volume %s from the database: %v", string(id), err)
				}
			} else {
				volumes = append(volumes, volume)
			}

			return nil
		})
		return err
	})
	if err != nil {
		return nil, err
	}

	return volumes, nil
}

// Volume retrieves a volume from full name
func (s *BoltState) Volume(name string) (*Volume, error) {
	if name == "" {
		return nil, define.ErrEmptyID
	}

	if !s.valid {
		return nil, define.ErrDBClosed
	}

	volName := []byte(name)

	volume := new(Volume)
	volume.config = new(VolumeConfig)
	volume.state = new(VolumeState)

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		volBkt, err := getVolBucket(tx)
		if err != nil {
			return err
		}

		return s.getVolumeFromDB(volName, volume, volBkt)
	})
	if err != nil {
		return nil, err
	}

	return volume, nil
}

// LookupVolume locates a volume from a partial name.
func (s *BoltState) LookupVolume(name string) (*Volume, error) {
	if name == "" {
		return nil, define.ErrEmptyID
	}

	if !s.valid {
		return nil, define.ErrDBClosed
	}

	volName := []byte(name)

	volume := new(Volume)
	volume.config = new(VolumeConfig)
	volume.state = new(VolumeState)

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		volBkt, err := getVolBucket(tx)
		if err != nil {
			return err
		}

		allVolsBkt, err := getAllVolsBucket(tx)
		if err != nil {
			return err
		}

		// Check for exact match on name
		volDB := volBkt.Bucket(volName)
		if volDB != nil {
			return s.getVolumeFromDB(volName, volume, volBkt)
		}

		// No exact match. Search all names.
		foundMatch := false
		err = allVolsBkt.ForEach(func(checkName, checkName2 []byte) error {
			if strings.HasPrefix(string(checkName), name) {
				if foundMatch {
					return errors.Wrapf(define.ErrVolumeExists, "more than one result for volume name %q", name)
				}
				foundMatch = true
				volName = checkName
			}
			return nil
		})
		if err != nil {
			return err
		}

		if !foundMatch {
			return errors.Wrapf(define.ErrNoSuchVolume, "no volume with name %q found", name)
		}

		return s.getVolumeFromDB(volName, volume, volBkt)
	})
	if err != nil {
		return nil, err
	}

	return volume, nil

}

// HasVolume returns true if the given volume exists in the state, otherwise it returns false
func (s *BoltState) HasVolume(name string) (bool, error) {
	if name == "" {
		return false, define.ErrEmptyID
	}

	if !s.valid {
		return false, define.ErrDBClosed
	}

	volName := []byte(name)

	exists := false

	db, err := s.getDBCon()
	if err != nil {
		return false, err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		volBkt, err := getVolBucket(tx)
		if err != nil {
			return err
		}

		volDB := volBkt.Bucket(volName)
		if volDB != nil {
			exists = true
		}

		return nil
	})
	if err != nil {
		return false, err
	}

	return exists, nil
}

// VolumeInUse checks if any container is using the volume
// It returns a slice of the IDs of the containers using the given
// volume. If the slice is empty, no containers use the given volume
func (s *BoltState) VolumeInUse(volume *Volume) ([]string, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	if !volume.valid {
		return nil, define.ErrVolumeRemoved
	}

	depCtrs := []string{}

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		volBucket, err := getVolBucket(tx)
		if err != nil {
			return err
		}

		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		volDB := volBucket.Bucket([]byte(volume.Name()))
		if volDB == nil {
			volume.valid = false
			return errors.Wrapf(define.ErrNoSuchVolume, "no volume with name %s found in DB", volume.Name())
		}

		dependsBkt := volDB.Bucket(volDependenciesBkt)
		if dependsBkt == nil {
			return errors.Wrapf(define.ErrInternal, "volume %s has no dependencies bucket", volume.Name())
		}

		// Iterate through and add dependencies
		err = dependsBkt.ForEach(func(id, value []byte) error {
			// Look up all dependencies and see that they
			// still exist before appending.
			ctrExists := ctrBucket.Bucket(id)
			if ctrExists == nil {
				return nil
			}

			depCtrs = append(depCtrs, string(id))

			return nil
		})
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return depCtrs, nil
}

// AddPod adds the given pod to the state.
func (s *BoltState) AddPod(pod *Pod) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !pod.valid {
		return define.ErrPodRemoved
	}

	if s.namespace != "" && s.namespace != pod.config.Namespace {
		return errors.Wrapf(define.ErrNSMismatch, "pod %s is in namespace %q but we are in namespace %q", pod.ID(), pod.config.Namespace, s.namespace)
	}

	podID := []byte(pod.ID())
	podName := []byte(pod.Name())

	var podNamespace []byte
	if pod.config.Namespace != "" {
		podNamespace = []byte(pod.config.Namespace)
	}

	podConfigJSON, err := json.Marshal(pod.config)
	if err != nil {
		return errors.Wrapf(err, "error marshalling pod %s config to JSON", pod.ID())
	}

	podStateJSON, err := json.Marshal(pod.state)
	if err != nil {
		return errors.Wrapf(err, "error marshalling pod %s state to JSON", pod.ID())
	}

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	err = db.Update(func(tx *bolt.Tx) error {
		podBkt, err := getPodBucket(tx)
		if err != nil {
			return err
		}

		allPodsBkt, err := getAllPodsBucket(tx)
		if err != nil {
			return err
		}

		idsBkt, err := getIDBucket(tx)
		if err != nil {
			return err
		}

		namesBkt, err := getNamesBucket(tx)
		if err != nil {
			return err
		}

		nsBkt, err := getNSBucket(tx)
		if err != nil {
			return err
		}

		// Check if we already have something with the given ID and name
		idExist := idsBkt.Get(podID)
		if idExist != nil {
			err = define.ErrPodExists
			if allPodsBkt.Get(idExist) == nil {
				err = define.ErrCtrExists
			}
			return errors.Wrapf(err, "ID \"%s\" is in use", pod.ID())
		}
		nameExist := namesBkt.Get(podName)
		if nameExist != nil {
			err = define.ErrPodExists
			if allPodsBkt.Get(nameExist) == nil {
				err = define.ErrCtrExists
			}
			return errors.Wrapf(err, "name \"%s\" is in use", pod.Name())
		}

		// We are good to add the pod
		// Make a bucket for it
		newPod, err := podBkt.CreateBucket(podID)
		if err != nil {
			return errors.Wrapf(err, "error creating bucket for pod %s", pod.ID())
		}

		// Make a subbucket for pod containers
		if _, err := newPod.CreateBucket(containersBkt); err != nil {
			return errors.Wrapf(err, "error creating bucket for pod %s containers", pod.ID())
		}

		if err := newPod.Put(configKey, podConfigJSON); err != nil {
			return errors.Wrapf(err, "error storing pod %s configuration in DB", pod.ID())
		}

		if err := newPod.Put(stateKey, podStateJSON); err != nil {
			return errors.Wrapf(err, "error storing pod %s state JSON in DB", pod.ID())
		}

		if podNamespace != nil {
			if err := newPod.Put(namespaceKey, podNamespace); err != nil {
				return errors.Wrapf(err, "error storing pod %s namespace in DB", pod.ID())
			}
			if err := nsBkt.Put(podID, podNamespace); err != nil {
				return errors.Wrapf(err, "error storing pod %s namespace in DB", pod.ID())
			}
		}

		// Add us to the ID and names buckets
		if err := idsBkt.Put(podID, podName); err != nil {
			return errors.Wrapf(err, "error storing pod %s ID in DB", pod.ID())
		}
		if err := namesBkt.Put(podName, podID); err != nil {
			return errors.Wrapf(err, "error storing pod %s name in DB", pod.Name())
		}
		if err := allPodsBkt.Put(podID, podName); err != nil {
			return errors.Wrapf(err, "error storing pod %s in all pods bucket in DB", pod.ID())
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// RemovePod removes the given pod from the state
// Only empty pods can be removed
func (s *BoltState) RemovePod(pod *Pod) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !pod.valid {
		return define.ErrPodRemoved
	}

	if s.namespace != "" && s.namespace != pod.config.Namespace {
		return errors.Wrapf(define.ErrNSMismatch, "pod %s is in namespace %q but we are in namespace %q", pod.ID(), pod.config.Namespace, s.namespace)
	}

	podID := []byte(pod.ID())
	podName := []byte(pod.Name())

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	err = db.Update(func(tx *bolt.Tx) error {
		podBkt, err := getPodBucket(tx)
		if err != nil {
			return err
		}

		allPodsBkt, err := getAllPodsBucket(tx)
		if err != nil {
			return err
		}

		idsBkt, err := getIDBucket(tx)
		if err != nil {
			return err
		}

		namesBkt, err := getNamesBucket(tx)
		if err != nil {
			return err
		}

		nsBkt, err := getNSBucket(tx)
		if err != nil {
			return err
		}

		// Check if the pod exists
		podDB := podBkt.Bucket(podID)
		if podDB == nil {
			pod.valid = false
			return errors.Wrapf(define.ErrNoSuchPod, "pod %s does not exist in DB", pod.ID())
		}

		// Check if pod is empty
		// This should never be nil
		// But if it is, we can assume there are no containers in the
		// pod.
		// So let's eject the malformed pod without error.
		podCtrsBkt := podDB.Bucket(containersBkt)
		if podCtrsBkt != nil {
			cursor := podCtrsBkt.Cursor()
			if id, _ := cursor.First(); id != nil {
				return errors.Wrapf(define.ErrCtrExists, "pod %s is not empty", pod.ID())
			}
		}

		// Pod is empty, and ready for removal
		// Let's kick it out
		if err := idsBkt.Delete(podID); err != nil {
			return errors.Wrapf(err, "error removing pod %s ID from DB", pod.ID())
		}
		if err := namesBkt.Delete(podName); err != nil {
			return errors.Wrapf(err, "error removing pod %s name (%s) from DB", pod.ID(), pod.Name())
		}
		if err := nsBkt.Delete(podID); err != nil {
			return errors.Wrapf(err, "error removing pod %s namespace from DB", pod.ID())
		}
		if err := allPodsBkt.Delete(podID); err != nil {
			return errors.Wrapf(err, "error removing pod %s ID from all pods bucket in DB", pod.ID())
		}
		if err := podBkt.DeleteBucket(podID); err != nil {
			return errors.Wrapf(err, "error removing pod %s from DB", pod.ID())
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// RemovePodContainers removes all containers in a pod
func (s *BoltState) RemovePodContainers(pod *Pod) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !pod.valid {
		return define.ErrPodRemoved
	}

	if s.namespace != "" && s.namespace != pod.config.Namespace {
		return errors.Wrapf(define.ErrNSMismatch, "pod %s is in namespace %q but we are in namespace %q", pod.ID(), pod.config.Namespace, s.namespace)
	}

	podID := []byte(pod.ID())

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	err = db.Update(func(tx *bolt.Tx) error {
		podBkt, err := getPodBucket(tx)
		if err != nil {
			return err
		}

		ctrBkt, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		allCtrsBkt, err := getAllCtrsBucket(tx)
		if err != nil {
			return err
		}

		idsBkt, err := getIDBucket(tx)
		if err != nil {
			return err
		}

		namesBkt, err := getNamesBucket(tx)
		if err != nil {
			return err
		}

		// Check if the pod exists
		podDB := podBkt.Bucket(podID)
		if podDB == nil {
			pod.valid = false
			return errors.Wrapf(define.ErrNoSuchPod, "pod %s does not exist in DB", pod.ID())
		}

		podCtrsBkt := podDB.Bucket(containersBkt)
		if podCtrsBkt == nil {
			return errors.Wrapf(define.ErrInternal, "pod %s does not have a containers bucket", pod.ID())
		}

		// Traverse all containers in the pod with a cursor
		// for-each has issues with data mutation
		err = podCtrsBkt.ForEach(func(id, name []byte) error {
			// Get the container so we can check dependencies
			ctr := ctrBkt.Bucket(id)
			if ctr == nil {
				// This should never happen
				// State is inconsistent
				return errors.Wrapf(define.ErrNoSuchCtr, "pod %s referenced nonexistent container %s", pod.ID(), string(id))
			}
			ctrDeps := ctr.Bucket(dependenciesBkt)
			// This should never be nil, but if it is, we're
			// removing it anyways, so continue if it is
			if ctrDeps != nil {
				err = ctrDeps.ForEach(func(depID, name []byte) error {
					exists := podCtrsBkt.Get(depID)
					if exists == nil {
						return errors.Wrapf(define.ErrCtrExists, "container %s has dependency %s outside of pod %s", string(id), string(depID), pod.ID())
					}
					return nil
				})
				if err != nil {
					return err
				}
			}

			// Dependencies are set, we're clear to remove

			if err := ctrBkt.DeleteBucket(id); err != nil {
				return errors.Wrapf(define.ErrInternal, "error deleting container %s from DB", string(id))
			}

			if err := idsBkt.Delete(id); err != nil {
				return errors.Wrapf(err, "error deleting container %s ID in DB", string(id))
			}

			if err := namesBkt.Delete(name); err != nil {
				return errors.Wrapf(err, "error deleting container %s name in DB", string(id))
			}

			if err := allCtrsBkt.Delete(id); err != nil {
				return errors.Wrapf(err, "error deleting container %s ID from all containers bucket in DB", string(id))
			}

			return nil
		})
		if err != nil {
			return err
		}

		// Delete and recreate the bucket to empty it
		if err := podDB.DeleteBucket(containersBkt); err != nil {
			return errors.Wrapf(err, "error removing pod %s containers bucket", pod.ID())
		}
		if _, err := podDB.CreateBucket(containersBkt); err != nil {
			return errors.Wrapf(err, "error recreating pod %s containers bucket", pod.ID())
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// AddContainerToPod adds the given container to an existing pod
// The container will be added to the state and the pod
func (s *BoltState) AddContainerToPod(pod *Pod, ctr *Container) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !pod.valid {
		return define.ErrPodRemoved
	}

	if !ctr.valid {
		return define.ErrCtrRemoved
	}

	if ctr.config.Pod != pod.ID() {
		return errors.Wrapf(define.ErrNoSuchCtr, "container %s is not part of pod %s", ctr.ID(), pod.ID())
	}

	return s.addContainer(ctr, pod)
}

// RemoveContainerFromPod removes a container from an existing pod
// The container will also be removed from the state
func (s *BoltState) RemoveContainerFromPod(pod *Pod, ctr *Container) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !pod.valid {
		return define.ErrPodRemoved
	}

	if s.namespace != "" {
		if s.namespace != pod.config.Namespace {
			return errors.Wrapf(define.ErrNSMismatch, "pod %s is in namespace %q but we are in namespace %q", pod.ID(), pod.config.Namespace, s.namespace)
		}
		if s.namespace != ctr.config.Namespace {
			return errors.Wrapf(define.ErrNSMismatch, "container %s in in namespace %q but we are in namespace %q", ctr.ID(), ctr.config.Namespace, s.namespace)
		}
	}

	if ctr.config.Pod == "" {
		return errors.Wrapf(define.ErrNoSuchPod, "container %s is not part of a pod, use RemoveContainer instead", ctr.ID())
	}

	if ctr.config.Pod != pod.ID() {
		return errors.Wrapf(define.ErrInvalidArg, "container %s is not part of pod %s", ctr.ID(), pod.ID())
	}

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	err = db.Update(func(tx *bolt.Tx) error {
		return s.removeContainer(ctr, pod, tx)
	})
	return err
}

// UpdatePod updates a pod's state from the database
func (s *BoltState) UpdatePod(pod *Pod) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !pod.valid {
		return define.ErrPodRemoved
	}

	if s.namespace != "" && s.namespace != pod.config.Namespace {
		return errors.Wrapf(define.ErrNSMismatch, "pod %s is in namespace %q but we are in namespace %q", pod.ID(), pod.config.Namespace, s.namespace)
	}

	newState := new(podState)

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	podID := []byte(pod.ID())

	err = db.View(func(tx *bolt.Tx) error {
		podBkt, err := getPodBucket(tx)
		if err != nil {
			return err
		}

		podDB := podBkt.Bucket(podID)
		if podDB == nil {
			pod.valid = false
			return errors.Wrapf(define.ErrNoSuchPod, "no pod with ID %s found in database", pod.ID())
		}

		// Get the pod state JSON
		podStateBytes := podDB.Get(stateKey)
		if podStateBytes == nil {
			return errors.Wrapf(define.ErrInternal, "pod %s is missing state key in DB", pod.ID())
		}

		if err := json.Unmarshal(podStateBytes, newState); err != nil {
			return errors.Wrapf(err, "error unmarshalling pod %s state JSON", pod.ID())
		}

		return nil
	})
	if err != nil {
		return err
	}

	pod.state = newState

	return nil
}

// SavePod saves a pod's state to the database
func (s *BoltState) SavePod(pod *Pod) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !pod.valid {
		return define.ErrPodRemoved
	}

	if s.namespace != "" && s.namespace != pod.config.Namespace {
		return errors.Wrapf(define.ErrNSMismatch, "pod %s is in namespace %q but we are in namespace %q", pod.ID(), pod.config.Namespace, s.namespace)
	}

	stateJSON, err := json.Marshal(pod.state)
	if err != nil {
		return errors.Wrapf(err, "error marshalling pod %s state to JSON", pod.ID())
	}

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	podID := []byte(pod.ID())

	err = db.Update(func(tx *bolt.Tx) error {
		podBkt, err := getPodBucket(tx)
		if err != nil {
			return err
		}

		podDB := podBkt.Bucket(podID)
		if podDB == nil {
			pod.valid = false
			return errors.Wrapf(define.ErrNoSuchPod, "no pod with ID %s found in database", pod.ID())
		}

		// Set the pod state JSON
		if err := podDB.Put(stateKey, stateJSON); err != nil {
			return errors.Wrapf(err, "error updating pod %s state in database", pod.ID())
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// AllPods returns all pods present in the state
func (s *BoltState) AllPods() ([]*Pod, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	pods := []*Pod{}

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		allPodsBucket, err := getAllPodsBucket(tx)
		if err != nil {
			return err
		}

		podBucket, err := getPodBucket(tx)
		if err != nil {
			return err
		}

		err = allPodsBucket.ForEach(func(id, name []byte) error {
			podExists := podBucket.Bucket(id)
			// This check can be removed if performance becomes an
			// issue, but much less helpful errors will be produced
			if podExists == nil {
				return errors.Wrapf(define.ErrInternal, "inconsistency in state - pod %s is in all pods bucket but pod not found", string(id))
			}

			pod := new(Pod)
			pod.config = new(PodConfig)
			pod.state = new(podState)

			if err := s.getPodFromDB(id, pod, podBucket); err != nil {
				if errors.Cause(err) != define.ErrNSMismatch {
					logrus.Errorf("Error retrieving pod %s from the database: %v", string(id), err)
				}
			} else {
				pods = append(pods, pod)
			}

			return nil
		})
		return err
	})
	if err != nil {
		return nil, err
	}

	return pods, nil
}
