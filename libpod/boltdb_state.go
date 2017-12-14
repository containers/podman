package libpod

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/pkg/errors"
)

// BoltState is a state implementation backed by a Bolt DB
type BoltState struct {
	valid   bool
	dbPath  string
	lockDir string
	runtime *Runtime
}

// NewBoltState creates a new bolt-backed state database
func NewBoltState(path, lockDir string, runtime *Runtime) (State, error) {
	state := new(BoltState)
	state.dbPath = path
	state.lockDir = lockDir
	state.runtime = runtime

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
	defer db.Close()

	// Perform initial database setup
	err = db.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists(idRegistryBkt)
		if err != nil {
			return errors.Wrapf(err, "error creating id-registry bucket")
		}
		_, err = tx.CreateBucketIfNotExists(nameRegistryBkt)
		if err != nil {
			return errors.Wrapf(err, "error creating name-registry bucket")
		}
		_, err = tx.CreateBucketIfNotExists(ctrConfigBkt)
		if err != nil {
			return errors.Wrapf(err, "error creating container-config bucket")
		}
		_, err = tx.CreateBucketIfNotExists(ctrStateBkt)
		if err != nil {
			return errors.Wrapf(err, "error creating container-state bucket")
		}
		_, err = tx.CreateBucketIfNotExists(netNSBkt)
		if err != nil {
			return errors.Wrapf(err, "error creating net-ns bucket")
		}
		_, err = tx.CreateBucketIfNotExists(runtimeConfigBkt)
		if err != nil {
			return errors.Wrapf(err, "error creating runtime-config bucket")
		}
		_, err = tx.CreateBucketIfNotExists(ctrDependsBkt)
		if err != nil {
			return errors.Wrapf(err, "error creating container-depends bucket")
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
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
		idBucket, err := getIDBucket(tx)
		if err != nil {
			return err
		}

		ctrStateBucket, err := getCtrStateBucket(tx)
		if err != nil {
			return err
		}

		netNSBucket, err := getNetNSBucket(tx)
		if err != nil {
			return err
		}

		// Iterate through all IDs. Check if they are containers.
		// If they are, unmarshal their state, and then clear
		// PID, mountpoint, and state for all of them
		// Then save the modified state
		// Also clear all network namespaces
		err = idBucket.ForEach(func(id, name []byte) error {
			if err := netNSBucket.Delete(id); err != nil {
				return errors.Wrapf(err, "error removing network namespace ID %s", string(id))
			}

			stateBytes := ctrStateBucket.Get(id)
			if stateBytes == nil {
				// This is a pod, not a container
				// Nothing to do
				return nil
			}

			state := new(containerState)

			if err := json.Unmarshal(stateBytes, state); err != nil {
				return errors.Wrapf(err, "error unmarshalling state for container %s", string(id))
			}

			state.PID = 0
			state.Mountpoint = ""
			state.Mounted = false
			state.State = ContainerStateConfigured

			newStateBytes, err := json.Marshal(state)
			if err != nil {
				return errors.Wrapf(err, "error marshalling modified state for container %s", string(id))
			}

			if err := ctrStateBucket.Put(id, newStateBytes); err != nil {
				return errors.Wrapf(err, "error updating state for container %s in DB", string(id))
			}

			return nil
		})
		return err
	})
	return err
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
	defer db.Close()

	err = db.View(func(tx *bolt.Tx) error {
		ctrConfigBucket, err := getCtrConfigBucket(tx)
		if err != nil {
			return err
		}

		ctrStateBucket, err := getCtrStateBucket(tx)
		if err != nil {
			return err
		}

		netNSBucket, err := getNetNSBucket(tx)
		if err != nil {
			return err
		}

		err = s.getContainerFromDB(ctrID, ctr, ctrConfigBucket,
			ctrStateBucket, netNSBucket)
		if err != nil {
			return err
		}

		return nil
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
	defer db.Close()

	err = db.View(func(tx *bolt.Tx) error {
		idBucket, err := getIDBucket(tx)
		if err != nil {
			return err
		}

		ctrConfigBucket, err := getCtrConfigBucket(tx)
		if err != nil {
			return err
		}

		ctrStateBucket, err := getCtrStateBucket(tx)
		if err != nil {
			return err
		}

		netNSBucket, err := getNetNSBucket(tx)
		if err != nil {
			return err
		}

		// First, check if the ID given was the actual container ID
		// Query against state because it's a lot smaller than config
		var id []byte
		stateExists := ctrStateBucket.Get([]byte(idOrName))
		if stateExists != nil {
			// A full container ID was given
			id = []byte(idOrName)
		} else {
			// They did not give us a full container ID.
			// Search for partial ID or full name matches
			// Use else-if in case the name is set to a partial ID
			exists := false
			err = idBucket.ForEach(func(checkID, checkName []byte) error {
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

		err = s.getContainerFromDB(id, ctr, ctrConfigBucket,
			ctrStateBucket, netNSBucket)
		if err != nil {
			return err
		}

		return nil
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
	defer db.Close()

	exists := false

	err = db.View(func(tx *bolt.Tx) error {
		idsBucket, err := getIDBucket(tx)
		if err != nil {
			return err
		}

		ctrExists := idsBucket.Get(ctrID)
		if ctrExists != nil {
			exists = true
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

	// Collect dependencies for the container. Use a map to ensure no dupes.
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

		ctrConfigBucket, err := getCtrConfigBucket(tx)
		if err != nil {
			return err
		}

		ctrStateBucket, err := getCtrStateBucket(tx)
		if err != nil {
			return err
		}

		netNSBucket, err := getNetNSBucket(tx)
		if err != nil {
			return err
		}

		ctrDependsBucket, err := getCtrDependsBucket(tx)
		if err != nil {
			return err
		}

		// Check if we already have a container with the given ID and name
		idExist := idsBucket.Get(ctrID)
		if idExist != nil {
			return errors.Wrapf(ErrCtrExists, "container with ID %s already exists", ctr.ID())
		}
		nameExist := namesBucket.Get(ctrName)
		if nameExist != nil {
			return errors.Wrapf(ErrCtrExists, "container with name %s already exists", ctr.Name())
		}

		// No overlapping containers
		// Add the new container to the DB
		if err := idsBucket.Put(ctrID, ctrName); err != nil {
			return errors.Wrapf(err, "error adding container %s ID to DB", ctr.ID())
		}
		if err := namesBucket.Put(ctrName, ctrID); err != nil {
			return errors.Wrapf(err, "error adding container %s name (%s) to DB", ctr.ID(), ctr.Name())
		}
		if err := ctrConfigBucket.Put(ctrID, configJSON); err != nil {
			return errors.Wrapf(err, "error adding container %s config to DB", ctr.ID())
		}
		if err := ctrStateBucket.Put(ctrID, stateJSON); err != nil {
			return errors.Wrapf(err, "error adding container %s state to DB", ctr.ID())
		}
		if netNSPath != "" {
			if err := netNSBucket.Put(ctrID, []byte(netNSPath)); err != nil {
				return errors.Wrapf(err, "error adding container %s netns path to DB", ctr.ID())
			}
		}

		// Add dependencies for the container
		for _, dependsCtr := range dependsCtrs {
			depCtrID := []byte(dependsCtr)
			deps := ctrDependsBucket.Get(depCtrID)
			depsArray := []string{}
			if deps != nil {
				if err := json.Unmarshal(deps, &depsArray); err != nil {
					return errors.Wrapf(err, "error unmarshalling container %s deps JSON", dependsCtr)
				}
			}
			depsArray = append(depsArray, ctr.ID())
			depsJSON, err := json.Marshal(&depsArray)
			if err != nil {
				return errors.Wrapf(err, "error marshalling container %s deps JSON", dependsCtr)
			}
			if err := ctrDependsBucket.Put(depCtrID, depsJSON); err != nil {
				return errors.Wrapf(err, "error adding container %s dependencies JSON to DB", dependsCtr)
			}
		}

		return nil
	})
	return err
}

// RemoveContainer removes a container from the state
// The container will only be removed from the state and not any pods it belongs
// to
func (s *BoltState) RemoveContainer(ctr *Container) error {
	if !s.valid {
		return ErrDBClosed
	}

	ctrID := []byte(ctr.ID())
	ctrName := []byte(ctr.Name())

	depCtrs := ctr.Dependencies()

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

		ctrConfigBucket, err := getCtrConfigBucket(tx)
		if err != nil {
			return err
		}

		ctrStateBucket, err := getCtrStateBucket(tx)
		if err != nil {
			return err
		}

		netNSBucket, err := getNetNSBucket(tx)
		if err != nil {
			return err
		}

		ctrDependsBucket, err := getCtrDependsBucket(tx)
		if err != nil {
			return err
		}

		// Does the container exist?
		ctrExists := idsBucket.Get(ctrID)
		if ctrExists == nil {
			return errors.Wrapf(ErrNoSuchCtr, "no container with ID %s found in DB", ctr.ID())
		}

		// Does the container have dependencies?
		ctrDeps := ctrDependsBucket.Get(ctrID)
		if ctrDeps != nil {
			dependsCtrs := []string{}
			if err := json.Unmarshal(ctrDeps, &dependsCtrs); err != nil {
				return errors.Wrapf(err, "cannot unmarshal container %s dependencies JSON", ctr.ID())
			}
			if len(dependsCtrs) > 0 {
				depsStr := strings.Join(dependsCtrs, ", ")

				return errors.Wrapf(ErrCtrExists, "container %s is a dependency on the following containers: %s", ctr.ID(), depsStr)
			}
		}

		if err := idsBucket.Delete(ctrID); err != nil {
			return errors.Wrapf(err, "error deleting container %s ID in DB", ctr.ID())
		}

		if err := namesBucket.Delete(ctrName); err != nil {
			return errors.Wrapf(err, "error deleting container %s name in DB", ctr.ID())
		}

		if err := ctrConfigBucket.Delete(ctrID); err != nil {
			return errors.Wrapf(err, "error deleting container %s config in DB", ctr.ID())
		}

		if err := ctrStateBucket.Delete(ctrID); err != nil {
			return errors.Wrapf(err, "error deleting container %s state in DB", ctr.ID())
		}

		// Can safely delete netNS even if it doesn't exist
		// Delete on a non-existent key doesn't error
		if err := netNSBucket.Delete(ctrID); err != nil {
			return errors.Wrapf(err, "error deleting container %s network ns in DB", ctr.ID())
		}

		// As above, can safely delete even if it doesn't exist
		if err := ctrDependsBucket.Delete(ctrID); err != nil {
			return errors.Wrapf(err, "error deleting container %s dependencies in DB", ctr.ID())
		}

		// Remove us from other container's dependencies
		for _, depCtr := range depCtrs {
			depCtrID := []byte(depCtr)
			dep := ctrDependsBucket.Get(depCtrID)
			if dep == nil {
				// Inconsistent state, but the dependency we were trying to remove doesn't exist...
				// Just continue
				continue
			}
			depStr := []string{}
			if err := json.Unmarshal(dep, &depStr); err != nil {
				return errors.Wrapf(err, "error unmarshaling ctr %s dependencies", ctr.ID(), depCtr)
			}
			newDeps := make([]string, 0, len(depStr))
			for _, checkID := range depStr {
				if checkID != ctr.ID() {
					newDeps = append(newDeps, checkID)
				}
			}
			if len(newDeps) == 0 {
				// Just delete the container's deps
				if err := ctrDependsBucket.Delete(depCtrID); err != nil {
					return errors.Wrapf(err, "error deleting container %s dependencies in DB", depCtr)
				}
			} else {
				// Store the new deps
				depsJSON, err := json.Marshal(&newDeps)
				if err != nil {
					return errors.Wrapf(err, "error marshalling container %s dependencies", depCtr)
				}
				if err := ctrDependsBucket.Put(depCtrID, depsJSON); err != nil {
					return errors.Wrapf(err, "error adding container %s dependencies to DB", depCtr)
				}
			}
		}

		return nil
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

	newState := new(containerState)
	netNSPath := ""

	ctrID := []byte(ctr.ID())

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer db.Close()

	err = db.View(func(tx *bolt.Tx) error {
		ctrStateBucket, err := getCtrStateBucket(tx)
		if err != nil {
			return err
		}

		netNSBucket, err := getNetNSBucket(tx)
		if err != nil {
			return err
		}

		newStateBytes := ctrStateBucket.Get(ctrID)
		if newStateBytes == nil {
			ctr.valid = false
			return errors.Wrapf(ErrCtrRemoved, "container %s removed from database", ctr.ID())
		}

		if err := json.Unmarshal(newStateBytes, newState); err != nil {
			return errors.Wrapf(err, "error unmarshalling container %s state", ctr.ID())
		}

		netNSBytes := netNSBucket.Get(ctrID)
		if netNSBytes != nil {
			netNSPath = string(netNSBytes)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Do we need to replace the container's netns?
	if netNSPath != "" {
		// Check if the container's old state has a good netns
		if ctr.state.NetNS != nil && netNSPath == ctr.state.NetNS.Path() {
			newState.NetNS = ctr.state.NetNS
		} else {
			// Tear down the existing namespace
			if err := s.runtime.teardownNetNS(ctr); err != nil {
				return err
			}

			// Open the new network namespace
			ns, err := joinNetNS(netNSPath)
			if err != nil {
				return errors.Wrapf(err, "error joining network namespace for container %s", ctr.ID())
			}
			newState.NetNS = ns
		}
	} else {
		// The container no longer has a network namespace
		// Tear down the old one
		if err := s.runtime.teardownNetNS(ctr); err != nil {
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
		return ErrDBClosed
	}

	if !ctr.valid {
		return ErrCtrRemoved
	}

	stateJSON, err := json.Marshal(ctr.state)
	if err != nil {
		return errors.Wrapf(err, "error marshalling container %s state to JSON", ctr.ID())
	}
	netNSPath := ""
	if ctr.state.NetNS != nil {
		netNSPath = ctr.state.NetNS.Path()
	}

	ctrID := []byte(ctr.ID())

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
		ctrStateBucket, err := getCtrStateBucket(tx)
		if err != nil {
			return err
		}

		netNSBucket, err := getNetNSBucket(tx)
		if err != nil {
			return err
		}

		oldJSON := ctrStateBucket.Get(ctrID)
		if oldJSON == nil {
			ctr.valid = false
			return errors.Wrapf(ErrCtrRemoved, "container %s no longer present in DB", ctr.ID())
		}

		// Update the state
		if err := ctrStateBucket.Put(ctrID, stateJSON); err != nil {
			return errors.Wrapf(err, "error updating container %s state in DB", ctr.ID())
		}

		if netNSPath != "" {
			if err := netNSBucket.Put(ctrID, []byte(netNSPath)); err != nil {
				return errors.Wrapf(err, "error updating network namespace path for container %s in DB", ctr.ID())
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

	depCtrs := []string{}

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	err = db.View(func(tx *bolt.Tx) error {
		ctrDependsBucket, err := getCtrDependsBucket(tx)
		if err != nil {
			return err
		}

		depsJSON := ctrDependsBucket.Get([]byte(ctr.ID()))
		if depsJSON == nil {
			// No deps, just return
			return nil
		}

		// We have deps, un-JSON them
		if err := json.Unmarshal(depsJSON, &depCtrs); err != nil {
			return errors.Wrapf(err, "error unmarshalling container %s dependencies", ctr.ID())
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
	defer db.Close()

	err = db.View(func(tx *bolt.Tx) error {
		ctrConfigBucket, err := getCtrConfigBucket(tx)
		if err != nil {
			return err
		}

		ctrStateBucket, err := getCtrStateBucket(tx)
		if err != nil {
			return err
		}

		netNSBucket, err := getNetNSBucket(tx)
		if err != nil {
			return err
		}

		// Iterate through all containers we know of in the state
		// Build a full container struct for all of them and append
		// it into the containers listing
		err = ctrStateBucket.ForEach(func(id, data []byte) error {
			ctr := new(Container)
			ctr.config = new(ContainerConfig)
			ctr.state = new(containerState)

			err = s.getContainerFromDB(id, ctr, ctrConfigBucket,
				ctrStateBucket, netNSBucket)
			if err != nil {
				return err
			}

			ctrs = append(ctrs, ctr)

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

// Pod retrieves a pod given its full ID
func (s *BoltState) Pod(id string) (*Pod, error) {
	return nil, ErrNotImplemented
}

// LookupPod retrieves a pod from full or unique partial ID or name
func (s *BoltState) LookupPod(idOrName string) (*Pod, error) {
	return nil, ErrNotImplemented
}

// HasPod checks if a pod with the given ID exists in the state
func (s *BoltState) HasPod(id string) (bool, error) {
	return false, ErrNotImplemented
}

// PodHasContainer checks if the given pod has a container with the given ID
func (s *BoltState) PodHasContainer(pod *Pod, id string) (bool, error) {
	return false, ErrNotImplemented
}

// PodContainersByID returns the IDs of all containers present in the given pod
func (s *BoltState) PodContainersByID(pod *Pod) ([]string, error) {
	return nil, ErrNotImplemented
}

// PodContainers returns all the containers present in the given pod
func (s *BoltState) PodContainers(pod *Pod) ([]*Container, error) {
	return nil, ErrNotImplemented
}

// AddPod adds the given pod to the state. Only empty pods can be added.
func (s *BoltState) AddPod(pod *Pod) error {
	return ErrNotImplemented
}

// RemovePod removes the given pod from the state
// Only empty pods can be removed
func (s *BoltState) RemovePod(pod *Pod) error {
	return ErrNotImplemented
}

// RemovePodContainers removes all containers in a pod
func (s *BoltState) RemovePodContainers(pod *Pod) error {
	return ErrNotImplemented
}

// AddContainerToPod adds the given container to an existing pod
// The container will be added to the state and the pod
func (s *BoltState) AddContainerToPod(pod *Pod, ctr *Container) error {
	return ErrNotImplemented
}

// RemoveContainerFromPod removes a container from an existing pod
// The container will also be removed from the state
func (s *BoltState) RemoveContainerFromPod(pod *Pod, ctr *Container) error {
	return ErrNotImplemented
}

// AllPods returns all pods present in the state
func (s *BoltState) AllPods() ([]*Pod, error) {
	return nil, ErrNotImplemented
}
