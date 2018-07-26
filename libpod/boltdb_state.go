package libpod

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// BoltState is a state implementation backed by a Bolt DB
type BoltState struct {
	valid          bool
	dbPath         string
	dbLock         sync.Mutex
	namespace      string
	namespaceBytes []byte
	lockDir        string
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
// - allCtrsBkt: Map of ID to name containing only containers. Used for
//   container lookup operations.
// - podBkt: Contains a sub-bucket for each pod in the state.
//   Each sub-bucket has config and state keys holding the pod's JSON encoded
//   configuration and state, plus a containers sub bucket holding the IDs of
//   containers in the pod.
// - allPodsBkt: Map of ID to name containing only pods. Used for pod lookup
//   operations.
// - runtimeConfigBkt: Contains configuration of the libpod instance that
//   initially created the database. This must match for any further instances
//   that access the database, to ensure that state mismatches with
//   containers/storage do not occur.

// NewBoltState creates a new bolt-backed state database
func NewBoltState(path, lockDir string, runtime *Runtime) (State, error) {
	state := new(BoltState)
	state.dbPath = path
	state.lockDir = lockDir
	state.runtime = runtime
	state.namespace = ""
	state.namespaceBytes = nil

	logrus.Debugf("Initializing boltdb state at %s", path)

	// Make the directory that will hold container lockfiles
	if err := os.MkdirAll(lockDir, 0750); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return nil, errors.Wrapf(err, "error creating lockfiles dir %s", lockDir)
		}
	}
	state.lockDir = lockDir

	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "error opening database %s", path)
	}
	// Everywhere else, we use s.closeDBCon(db) to ensure the state's DB
	// mutex is also unlocked.
	// However, here, the mutex has not been locked, since we just created
	// the DB connection, and it hasn't left this function yet - no risk of
	// concurrent access.
	// As such, just a db.Close() is fine here.
	defer db.Close()

	// Perform initial database setup
	err = db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(idRegistryBkt); err != nil {
			return errors.Wrapf(err, "error creating id-registry bucket")
		}
		if _, err := tx.CreateBucketIfNotExists(nameRegistryBkt); err != nil {
			return errors.Wrapf(err, "error creating name-registry bucket")
		}
		if _, err := tx.CreateBucketIfNotExists(nsRegistryBkt); err != nil {
			return errors.Wrapf(err, "error creating ns-registry bucket")
		}
		if _, err := tx.CreateBucketIfNotExists(ctrBkt); err != nil {
			return errors.Wrapf(err, "error creating containers bucket")
		}
		if _, err := tx.CreateBucketIfNotExists(allCtrsBkt); err != nil {
			return errors.Wrapf(err, "error creating all containers bucket")
		}
		if _, err := tx.CreateBucketIfNotExists(podBkt); err != nil {
			return errors.Wrapf(err, "error creating pods bucket")
		}
		if _, err := tx.CreateBucketIfNotExists(allPodsBkt); err != nil {
			return errors.Wrapf(err, "error creating all pods bucket")
		}
		if _, err := tx.CreateBucketIfNotExists(runtimeConfigBkt); err != nil {
			return errors.Wrapf(err, "error creating runtime-config bucket")
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "error creating initial database layout")
	}

	// Check runtime configuration
	if err := checkRuntimeConfig(db, runtime); err != nil {
		return nil, err
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
		return ErrDBClosed
	}

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.closeDBCon(db)

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
					return errors.Wrapf(ErrInternal, "id %s is not a pod or a container", string(id))
				}

				// Get the state
				stateBytes := podBkt.Get(stateKey)
				if stateBytes == nil {
					return errors.Wrapf(ErrInternal, "pod %s missing state key", string(id))
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
				return errors.Wrapf(ErrInternal, "container %s missing state in DB", string(id))
			}

			state := new(containerState)

			if err := json.Unmarshal(stateBytes, state); err != nil {
				return errors.Wrapf(err, "error unmarshalling state for container %s", string(id))
			}

			if err := resetState(state); err != nil {
				return errors.Wrapf(err, "error resetting state for container %s", string(id))
			}

			newStateBytes, err := json.Marshal(state)
			if err != nil {
				return errors.Wrapf(err, "error marshalling modified state for container %s", string(id))
			}

			if err := ctrBkt.Put(stateKey, newStateBytes); err != nil {
				return errors.Wrapf(err, "error updating state for container %s in DB", string(id))
			}

			return nil
		})
		return err
	})
	return err
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

// Container retrieves a single container from the state by its full ID
func (s *BoltState) Container(id string) (*Container, error) {
	if id == "" {
		return nil, ErrEmptyID
	}

	if !s.valid {
		return nil, ErrDBClosed
	}

	ctrID := []byte(id)

	ctr := new(Container)
	ctr.config = new(ContainerConfig)
	ctr.state = new(containerState)

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.closeDBCon(db)

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

// LookupContainer retrieves a container from the state by full or unique
// partial ID or name
func (s *BoltState) LookupContainer(idOrName string) (*Container, error) {
	if idOrName == "" {
		return nil, ErrEmptyID
	}

	if !s.valid {
		return nil, ErrDBClosed
	}

	ctr := new(Container)
	ctr.config = new(ContainerConfig)
	ctr.state = new(containerState)

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.closeDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		idBucket, err := getIDBucket(tx)
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

		// First, check if the ID given was the actual container ID
		var id []byte
		ctrExists := ctrBucket.Bucket([]byte(idOrName))
		if ctrExists != nil {
			// A full container ID was given.
			// It might not be in our namespace, but
			// getContainerFromDB() will handle that case.
			id = []byte(idOrName)
		} else {
			// They did not give us a full container ID.
			// Search for partial ID or full name matches
			// Use else-if in case the name is set to a partial ID
			exists := false
			err = idBucket.ForEach(func(checkID, checkName []byte) error {
				// If the container isn't in our namespace, we
				// can't match it
				if s.namespaceBytes != nil {
					ns := nsBucket.Get(checkID)
					if !bytes.Equal(ns, s.namespaceBytes) {
						return nil
					}
				}
				if string(checkName) == idOrName {
					if exists {
						return errors.Wrapf(ErrCtrExists, "more than one result for ID or name %s", idOrName)
					}
					id = checkID
					exists = true
				} else if strings.HasPrefix(string(checkID), idOrName) {
					if exists {
						return errors.Wrapf(ErrCtrExists, "more than one result for ID or name %s", idOrName)
					}
					id = checkID
					exists = true
				}

				return nil
			})
			if err != nil {
				return err
			} else if !exists {
				return errors.Wrapf(ErrNoSuchCtr, "no container with name or ID %s found", idOrName)
			}
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
		return false, ErrEmptyID
	}

	if !s.valid {
		return false, ErrDBClosed
	}

	ctrID := []byte(id)

	db, err := s.getDBCon()
	if err != nil {
		return false, err
	}
	defer s.closeDBCon(db)

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
		return ErrDBClosed
	}

	if !ctr.valid {
		return ErrCtrRemoved
	}

	if ctr.config.Pod != "" {
		return errors.Wrapf(ErrInvalidArg, "cannot add a container that belongs to a pod with AddContainer - use AddContainerToPod")
	}

	return s.addContainer(ctr, nil)
}

// RemoveContainer removes a container from the state
// Only removes containers not in pods - for containers that are a member of a
// pod, use RemoveContainerFromPod
func (s *BoltState) RemoveContainer(ctr *Container) error {
	if !s.valid {
		return ErrDBClosed
	}

	if ctr.config.Pod != "" {
		return errors.Wrapf(ErrPodExists, "container %s is part of a pod, use RemoveContainerFromPod instead", ctr.ID())
	}

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.closeDBCon(db)

	err = db.Update(func(tx *bolt.Tx) error {
		return s.removeContainer(ctr, nil, tx)
	})
	return err
}

// UpdateContainer updates a container's state from the database
func (s *BoltState) UpdateContainer(ctr *Container) error {
	if !s.valid {
		return ErrDBClosed
	}

	if !ctr.valid {
		return ErrCtrRemoved
	}

	if s.namespace != "" && s.namespace != ctr.config.Namespace {
		return errors.Wrapf(ErrNSMismatch, "container %s is in namespace %q, does not match our namespace %q", ctr.ID(), ctr.config.Namespace, s.namespace)
	}

	newState := new(containerState)
	netNSPath := ""

	ctrID := []byte(ctr.ID())

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.closeDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		ctrToUpdate := ctrBucket.Bucket(ctrID)
		if ctrToUpdate == nil {
			ctr.valid = false
			return errors.Wrapf(ErrNoSuchCtr, "container %s does not exist in database", ctr.ID())
		}

		newStateBytes := ctrToUpdate.Get(stateKey)
		if newStateBytes == nil {
			return errors.Wrapf(ErrInternal, "container %s does not have a state key in DB", ctr.ID())
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

	// Do we need to replace the container's netns?
	ctr.setNamespace(netNSPath, newState)

	// New state compiled successfully, swap it into the current state
	ctr.state = newState

	return nil
}

// SaveContainer saves a container's current state in the database
func (s *BoltState) SaveContainer(ctr *Container) error {
	if !s.valid {
		return ErrDBClosed
	}

	if !ctr.valid {
		return ErrCtrRemoved
	}

	if s.namespace != "" && s.namespace != ctr.config.Namespace {
		return errors.Wrapf(ErrNSMismatch, "container %s is in namespace %q, does not match our namespace %q", ctr.ID(), ctr.config.Namespace, s.namespace)
	}

	stateJSON, err := json.Marshal(ctr.state)
	if err != nil {
		return errors.Wrapf(err, "error marshalling container %s state to JSON", ctr.ID())
	}
	netNSPath := ctr.setNamespaceStatePath()

	ctrID := []byte(ctr.ID())

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.closeDBCon(db)

	err = db.Update(func(tx *bolt.Tx) error {
		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		ctrToSave := ctrBucket.Bucket(ctrID)
		if ctrToSave == nil {
			ctr.valid = false
			return errors.Wrapf(ErrNoSuchCtr, "container %s does not exist in DB", ctr.ID())
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
		return nil, ErrDBClosed
	}

	if !ctr.valid {
		return nil, ErrCtrRemoved
	}

	if s.namespace != "" && s.namespace != ctr.config.Namespace {
		return nil, errors.Wrapf(ErrNSMismatch, "container %s is in namespace %q, does not match our namespace %q", ctr.ID(), ctr.config.Namespace, s.namespace)
	}

	depCtrs := []string{}

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.closeDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		ctrDB := ctrBucket.Bucket([]byte(ctr.ID()))
		if ctrDB == nil {
			ctr.valid = false
			return errors.Wrapf(ErrNoSuchCtr, "no container with ID %s found in DB", ctr.ID())
		}

		dependsBkt := ctrDB.Bucket(dependenciesBkt)
		if dependsBkt == nil {
			return errors.Wrapf(ErrInternal, "container %s has no dependencies bucket", ctr.ID())
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
		return nil, ErrDBClosed
	}

	ctrs := []*Container{}

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.closeDBCon(db)

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
				return errors.Wrapf(ErrInternal, "state is inconsistent - container ID %s in all containers, but container not found", string(id))
			}

			ctr := new(Container)
			ctr.config = new(ContainerConfig)
			ctr.state = new(containerState)

			if err := s.getContainerFromDB(id, ctr, ctrBucket); err != nil {
				// If the error is a namespace mismatch, we can
				// ignore it safely.
				// We just won't include the container in the
				// results.
				if errors.Cause(err) != ErrNSMismatch {
					return err
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

// Pod retrieves a pod given its full ID
func (s *BoltState) Pod(id string) (*Pod, error) {
	if id == "" {
		return nil, ErrEmptyID
	}

	if !s.valid {
		return nil, ErrDBClosed
	}

	podID := []byte(id)

	pod := new(Pod)
	pod.config = new(PodConfig)
	pod.state = new(podState)

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.closeDBCon(db)

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
		return nil, ErrEmptyID
	}

	if !s.valid {
		return nil, ErrDBClosed
	}

	pod := new(Pod)
	pod.config = new(PodConfig)
	pod.state = new(podState)

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.closeDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		idBucket, err := getIDBucket(tx)
		if err != nil {
			return err
		}

		podBkt, err := getPodBucket(tx)
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
		} else {
			// They did not give us a full pod ID.
			// Search for partial ID or full name matches
			// Use else-if in case the name is set to a partial ID
			exists := false
			err = idBucket.ForEach(func(checkID, checkName []byte) error {
				// If the pod isn't in our namespace, we
				// can't match it
				if s.namespaceBytes != nil {
					ns := nsBkt.Get(checkID)
					if !bytes.Equal(ns, s.namespaceBytes) {
						return nil
					}
				}
				if string(checkName) == idOrName {
					if exists {
						return errors.Wrapf(ErrPodExists, "more than one result for ID or name %s", idOrName)
					}
					id = checkID
					exists = true
				} else if strings.HasPrefix(string(checkID), idOrName) {
					if exists {
						return errors.Wrapf(ErrPodExists, "more than one result for ID or name %s", idOrName)
					}
					id = checkID
					exists = true
				}

				return nil
			})
			if err != nil {
				return err
			} else if !exists {
				return errors.Wrapf(ErrNoSuchPod, "no pod with name or ID %s found", idOrName)
			}
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
		return false, ErrEmptyID
	}

	if !s.valid {
		return false, ErrDBClosed
	}

	podID := []byte(id)

	exists := false

	db, err := s.getDBCon()
	if err != nil {
		return false, err
	}
	defer s.closeDBCon(db)

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
		return false, ErrEmptyID
	}

	if !s.valid {
		return false, ErrDBClosed
	}

	if !pod.valid {
		return false, ErrPodRemoved
	}

	if s.namespace != "" && s.namespace != pod.config.Namespace {
		return false, errors.Wrapf(ErrNSMismatch, "pod %s is in namespace %q but we are in namespace %q", pod.ID(), pod.config.Namespace, s.namespace)
	}

	ctrID := []byte(id)
	podID := []byte(pod.ID())

	exists := false

	db, err := s.getDBCon()
	if err != nil {
		return false, err
	}
	defer s.closeDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		podBkt, err := getPodBucket(tx)
		if err != nil {
			return err
		}

		// Get pod itself
		podDB := podBkt.Bucket(podID)
		if podDB == nil {
			pod.valid = false
			return errors.Wrapf(ErrNoSuchPod, "pod %s not found in database", pod.ID())
		}

		// Get pod containers bucket
		podCtrs := podDB.Bucket(containersBkt)
		if podCtrs == nil {
			return errors.Wrapf(ErrInternal, "pod %s missing containers bucket in DB", pod.ID())
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
		return nil, ErrDBClosed
	}

	if !pod.valid {
		return nil, ErrPodRemoved
	}

	if s.namespace != "" && s.namespace != pod.config.Namespace {
		return nil, errors.Wrapf(ErrNSMismatch, "pod %s is in namespace %q but we are in namespace %q", pod.ID(), pod.config.Namespace, s.namespace)
	}

	podID := []byte(pod.ID())

	ctrs := []string{}

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.closeDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		podBkt, err := getPodBucket(tx)
		if err != nil {
			return err
		}

		// Get pod itself
		podDB := podBkt.Bucket(podID)
		if podDB == nil {
			pod.valid = false
			return errors.Wrapf(ErrNoSuchPod, "pod %s not found in database", pod.ID())
		}

		// Get pod containers bucket
		podCtrs := podDB.Bucket(containersBkt)
		if podCtrs == nil {
			return errors.Wrapf(ErrInternal, "pod %s missing containers bucket in DB", pod.ID())
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
		return nil, ErrDBClosed
	}

	if !pod.valid {
		return nil, ErrPodRemoved
	}

	if s.namespace != "" && s.namespace != pod.config.Namespace {
		return nil, errors.Wrapf(ErrNSMismatch, "pod %s is in namespace %q but we are in namespace %q", pod.ID(), pod.config.Namespace, s.namespace)
	}

	podID := []byte(pod.ID())

	ctrs := []*Container{}

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.closeDBCon(db)

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
			return errors.Wrapf(ErrNoSuchPod, "pod %s not found in database", pod.ID())
		}

		// Get pod containers bucket
		podCtrs := podDB.Bucket(containersBkt)
		if podCtrs == nil {
			return errors.Wrapf(ErrInternal, "pod %s missing containers bucket in DB", pod.ID())
		}

		// Iterate through all containers in the pod
		err = podCtrs.ForEach(func(id, val []byte) error {
			newCtr := new(Container)
			newCtr.config = new(ContainerConfig)
			newCtr.state = new(containerState)
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

// AddPod adds the given pod to the state.
func (s *BoltState) AddPod(pod *Pod) error {
	if !s.valid {
		return ErrDBClosed
	}

	if !pod.valid {
		return ErrPodRemoved
	}

	if s.namespace != "" && s.namespace != pod.config.Namespace {
		return errors.Wrapf(ErrNSMismatch, "pod %s is in namespace %q but we are in namespace %q", pod.ID(), pod.config.Namespace, s.namespace)
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
	defer s.closeDBCon(db)

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
			return errors.Wrapf(ErrPodExists, "ID %s is in use", pod.ID())
		}
		nameExist := namesBkt.Get(podName)
		if nameExist != nil {
			return errors.Wrapf(ErrPodExists, "name %s is in use", pod.Name())
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
		return ErrDBClosed
	}

	if !pod.valid {
		return ErrPodRemoved
	}

	if s.namespace != "" && s.namespace != pod.config.Namespace {
		return errors.Wrapf(ErrNSMismatch, "pod %s is in namespace %q but we are in namespace %q", pod.ID(), pod.config.Namespace, s.namespace)
	}

	podID := []byte(pod.ID())
	podName := []byte(pod.Name())

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.closeDBCon(db)

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
			return errors.Wrapf(ErrNoSuchPod, "pod %s does not exist in DB", pod.ID())
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
				return errors.Wrapf(ErrCtrExists, "pod %s is not empty", pod.ID())
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
		return ErrDBClosed
	}

	if !pod.valid {
		return ErrPodRemoved
	}

	if s.namespace != "" && s.namespace != pod.config.Namespace {
		return errors.Wrapf(ErrNSMismatch, "pod %s is in namespace %q but we are in namespace %q", pod.ID(), pod.config.Namespace, s.namespace)
	}

	podID := []byte(pod.ID())

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.closeDBCon(db)

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
			return errors.Wrapf(ErrNoSuchPod, "pod %s does not exist in DB", pod.ID())
		}

		podCtrsBkt := podDB.Bucket(containersBkt)
		if podCtrsBkt == nil {
			return errors.Wrapf(ErrInternal, "pod %s does not have a containers bucket", pod.ID())
		}

		// Traverse all containers in the pod with a cursor
		// for-each has issues with data mutation
		err = podCtrsBkt.ForEach(func(id, name []byte) error {
			// Get the container so we can check dependencies
			ctr := ctrBkt.Bucket(id)
			if ctr == nil {
				// This should never happen
				// State is inconsistent
				return errors.Wrapf(ErrNoSuchCtr, "pod %s referenced nonexistant container %s", pod.ID(), string(id))
			}
			ctrDeps := ctr.Bucket(dependenciesBkt)
			// This should never be nil, but if it is, we're
			// removing it anyways, so continue if it is
			if ctrDeps != nil {
				err = ctrDeps.ForEach(func(depID, name []byte) error {
					exists := podCtrsBkt.Get(depID)
					if exists == nil {
						return errors.Wrapf(ErrCtrExists, "container %s has dependency %s outside of pod %s", string(id), string(depID), pod.ID())
					}
					return nil
				})
				if err != nil {
					return err
				}
			}

			// Dependencies are set, we're clear to remove

			if err := ctrBkt.DeleteBucket(id); err != nil {
				return errors.Wrapf(ErrInternal, "error deleting container %s from DB", string(id))
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
		return ErrDBClosed
	}

	if !pod.valid {
		return ErrPodRemoved
	}

	if !ctr.valid {
		return ErrCtrRemoved
	}

	if ctr.config.Pod != pod.ID() {
		return errors.Wrapf(ErrNoSuchCtr, "container %s is not part of pod %s", ctr.ID(), pod.ID())
	}

	return s.addContainer(ctr, pod)
}

// RemoveContainerFromPod removes a container from an existing pod
// The container will also be removed from the state
func (s *BoltState) RemoveContainerFromPod(pod *Pod, ctr *Container) error {
	if !s.valid {
		return ErrDBClosed
	}

	if !pod.valid {
		return ErrPodRemoved
	}

	if s.namespace != "" {
		if s.namespace != pod.config.Namespace {
			return errors.Wrapf(ErrNSMismatch, "pod %s is in namespace %q but we are in namespace %q", pod.ID(), pod.config.Namespace, s.namespace)
		}
		if s.namespace != ctr.config.Namespace {
			return errors.Wrapf(ErrNSMismatch, "container %s in in namespace %q but we are in namespace %q", ctr.ID(), ctr.config.Namespace, s.namespace)
		}
	}

	if ctr.config.Pod == "" {
		return errors.Wrapf(ErrNoSuchPod, "container %s is not part of a pod, use RemoveContainer instead", ctr.ID())
	}

	if ctr.config.Pod != pod.ID() {
		return errors.Wrapf(ErrInvalidArg, "container %s is not part of pod %s", ctr.ID(), pod.ID())
	}

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.closeDBCon(db)

	err = db.Update(func(tx *bolt.Tx) error {
		return s.removeContainer(ctr, pod, tx)
	})
	return err
}

// UpdatePod updates a pod's state from the database
func (s *BoltState) UpdatePod(pod *Pod) error {
	if !s.valid {
		return ErrDBClosed
	}

	if !pod.valid {
		return ErrPodRemoved
	}

	if s.namespace != "" && s.namespace != pod.config.Namespace {
		return errors.Wrapf(ErrNSMismatch, "pod %s is in namespace %q but we are in namespace %q", pod.ID(), pod.config.Namespace, s.namespace)
	}

	newState := new(podState)

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.closeDBCon(db)

	podID := []byte(pod.ID())

	err = db.View(func(tx *bolt.Tx) error {
		podBkt, err := getPodBucket(tx)
		if err != nil {
			return err
		}

		podDB := podBkt.Bucket(podID)
		if podDB == nil {
			pod.valid = false
			return errors.Wrapf(ErrNoSuchPod, "no pod with ID %s found in database", pod.ID())
		}

		// Get the pod state JSON
		podStateBytes := podDB.Get(stateKey)
		if podStateBytes == nil {
			return errors.Wrapf(ErrInternal, "pod %s is missing state key in DB", pod.ID())
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
		return ErrDBClosed
	}

	if !pod.valid {
		return ErrPodRemoved
	}

	if s.namespace != "" && s.namespace != pod.config.Namespace {
		return errors.Wrapf(ErrNSMismatch, "pod %s is in namespace %q but we are in namespace %q", pod.ID(), pod.config.Namespace, s.namespace)
	}

	stateJSON, err := json.Marshal(pod.state)
	if err != nil {
		return errors.Wrapf(err, "error marshalling pod %s state to JSON", pod.ID())
	}

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.closeDBCon(db)

	podID := []byte(pod.ID())

	err = db.Update(func(tx *bolt.Tx) error {
		podBkt, err := getPodBucket(tx)
		if err != nil {
			return err
		}

		podDB := podBkt.Bucket(podID)
		if podDB == nil {
			pod.valid = false
			return errors.Wrapf(ErrNoSuchPod, "no pod with ID %s found in database", pod.ID())
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
		return nil, ErrDBClosed
	}

	pods := []*Pod{}

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.closeDBCon(db)

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
				return errors.Wrapf(ErrInternal, "inconsistency in state - pod %s is in all pods bucket but pod not found", string(id))
			}

			pod := new(Pod)
			pod.config = new(PodConfig)
			pod.state = new(podState)

			if err := s.getPodFromDB(id, pod, podBucket); err != nil {
				if errors.Cause(err) != ErrNSMismatch {
					return err
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
