package libpod

import (
	"strings"

	"github.com/containers/libpod/pkg/registrar"
	"github.com/containers/storage/pkg/truncindex"
	"github.com/pkg/errors"
)

// TODO: Maybe separate idIndex for pod/containers
// As of right now, partial IDs used in Lookup... need to be unique as well
// This may be undesirable?

// An InMemoryState is a purely in-memory state store
type InMemoryState struct {
	// Maps pod ID to pod struct.
	pods map[string]*Pod
	// Maps container ID to container struct.
	containers map[string]*Container
	volumes    map[string]*Volume
	// Maps container ID to a list of IDs of dependencies.
	ctrDepends    map[string][]string
	volumeDepends map[string][]string
	// Maps pod ID to a map of container ID to container struct.
	podContainers map[string]map[string]*Container
	// Global name registry - ensures name uniqueness and performs lookups.
	nameIndex *registrar.Registrar
	// Global ID registry - ensures ID uniqueness and performs lookups.
	idIndex *truncindex.TruncIndex
	// Namespace the state is joined to.
	namespace string
	// Maps namespace name to local ID and name registries for looking up
	// pods and containers in a specific namespace.
	namespaceIndexes map[string]*namespaceIndex
}

// namespaceIndex contains name and ID registries for a specific namespace.
// This is used for namespaces lookup operations.
type namespaceIndex struct {
	nameIndex *registrar.Registrar
	idIndex   *truncindex.TruncIndex
}

// NewInMemoryState initializes a new, empty in-memory state
func NewInMemoryState() (State, error) {
	state := new(InMemoryState)

	state.pods = make(map[string]*Pod)
	state.containers = make(map[string]*Container)
	state.volumes = make(map[string]*Volume)

	state.ctrDepends = make(map[string][]string)
	state.volumeDepends = make(map[string][]string)

	state.podContainers = make(map[string]map[string]*Container)

	state.nameIndex = registrar.NewRegistrar()
	state.idIndex = truncindex.NewTruncIndex([]string{})

	state.namespace = ""

	state.namespaceIndexes = make(map[string]*namespaceIndex)

	return state, nil
}

// Close the state before shutdown
// This is a no-op as we have no backing disk
func (s *InMemoryState) Close() error {
	return nil
}

// Refresh clears container and pod stats after a reboot
// In-memory state won't survive a reboot so this is a no-op
func (s *InMemoryState) Refresh() error {
	return nil
}

// GetDBConfig is not implemented for in-memory state.
// As we do not store a config, return an empty one.
func (s *InMemoryState) GetDBConfig() (*DBConfig, error) {
	return &DBConfig{}, nil
}

// ValidateDBConfig is not implemented for the in-memory state.
// Since we do nothing just return no error.
func (s *InMemoryState) ValidateDBConfig(runtime *Runtime) error {
	return nil
}

// SetNamespace sets the namespace for container and pod retrieval.
func (s *InMemoryState) SetNamespace(ns string) error {
	s.namespace = ns

	return nil
}

// Container retrieves a container from its full ID
func (s *InMemoryState) Container(id string) (*Container, error) {
	if id == "" {
		return nil, ErrEmptyID
	}

	ctr, ok := s.containers[id]
	if !ok {
		return nil, errors.Wrapf(ErrNoSuchCtr, "no container with ID %s found", id)
	}

	if err := s.checkNSMatch(ctr.ID(), ctr.Namespace()); err != nil {
		return nil, err
	}

	return ctr, nil
}

// LookupContainer retrieves a container by full ID, unique partial ID, or name
func (s *InMemoryState) LookupContainer(idOrName string) (*Container, error) {
	var (
		nameIndex *registrar.Registrar
		idIndex   *truncindex.TruncIndex
	)

	if idOrName == "" {
		return nil, ErrEmptyID
	}

	if s.namespace != "" {
		nsIndex, ok := s.namespaceIndexes[s.namespace]
		if !ok {
			// We have no containers in the namespace
			// Return false
			return nil, errors.Wrapf(ErrNoSuchCtr, "no container found with name or ID %s", idOrName)
		}
		nameIndex = nsIndex.nameIndex
		idIndex = nsIndex.idIndex
	} else {
		nameIndex = s.nameIndex
		idIndex = s.idIndex
	}

	fullID, err := nameIndex.Get(idOrName)
	if err != nil {
		if err == registrar.ErrNameNotReserved {
			// What was passed is not a name, assume it's an ID
			fullID, err = idIndex.Get(idOrName)
			if err != nil {
				if err == truncindex.ErrNotExist {
					return nil, errors.Wrapf(ErrNoSuchCtr, "no container found with name or ID %s", idOrName)
				}
				return nil, errors.Wrapf(err, "error performing truncindex lookup for ID %s", idOrName)
			}
		} else {
			return nil, errors.Wrapf(err, "error performing registry lookup for ID %s", idOrName)
		}
	}

	ctr, ok := s.containers[fullID]
	if !ok {
		// It's a pod, not a container
		return nil, errors.Wrapf(ErrNoSuchCtr, "name or ID %s is a pod, not a container", idOrName)
	}

	return ctr, nil
}

// HasContainer checks if a container with the given ID is present in the state
func (s *InMemoryState) HasContainer(id string) (bool, error) {
	if id == "" {
		return false, ErrEmptyID
	}

	ctr, ok := s.containers[id]
	if !ok || (s.namespace != "" && s.namespace != ctr.config.Namespace) {
		return false, nil
	}

	return true, nil
}

// AddContainer adds a container to the state
// Containers in a pod cannot be added to the state
func (s *InMemoryState) AddContainer(ctr *Container) error {
	if !ctr.valid {
		return errors.Wrapf(ErrCtrRemoved, "container with ID %s is not valid", ctr.ID())
	}

	if _, ok := s.containers[ctr.ID()]; ok {
		return errors.Wrapf(ErrCtrExists, "container with ID %s already exists in state", ctr.ID())
	}

	if ctr.config.Pod != "" {
		return errors.Wrapf(ErrInvalidArg, "cannot add a container that is in a pod with AddContainer, use AddContainerToPod")
	}

	if err := s.checkNSMatch(ctr.ID(), ctr.Namespace()); err != nil {
		return err
	}

	// There are potential race conditions with this
	// But in-memory state is intended purely for testing and not production
	// use, so this should be fine.
	depCtrs := ctr.Dependencies()
	for _, depID := range depCtrs {
		depCtr, ok := s.containers[depID]
		if !ok {
			return errors.Wrapf(ErrNoSuchCtr, "cannot depend on nonexistent container %s", depID)
		} else if depCtr.config.Pod != "" {
			return errors.Wrapf(ErrInvalidArg, "cannot depend on container in a pod if not part of same pod")
		}
		if depCtr.config.Namespace != ctr.config.Namespace {
			return errors.Wrapf(ErrNSMismatch, "container %s is in namespace %s and cannot depend on container %s in namespace %s", ctr.ID(), ctr.config.Namespace, depID, depCtr.config.Namespace)
		}
	}

	if err := s.nameIndex.Reserve(ctr.Name(), ctr.ID()); err != nil {
		return errors.Wrapf(err, "error registering container name %s", ctr.Name())
	}

	if err := s.idIndex.Add(ctr.ID()); err != nil {
		s.nameIndex.Release(ctr.Name())
		return errors.Wrapf(err, "error registering container ID %s", ctr.ID())
	}

	s.containers[ctr.ID()] = ctr

	// If we're in a namespace, add us to that namespace's indexes
	if ctr.config.Namespace != "" {
		var nsIndex *namespaceIndex
		nsIndex, ok := s.namespaceIndexes[ctr.config.Namespace]
		if !ok {
			nsIndex = new(namespaceIndex)
			nsIndex.nameIndex = registrar.NewRegistrar()
			nsIndex.idIndex = truncindex.NewTruncIndex([]string{})
			s.namespaceIndexes[ctr.config.Namespace] = nsIndex
		}
		// Should be no errors here, the previous index adds should have caught that
		if err := nsIndex.nameIndex.Reserve(ctr.Name(), ctr.ID()); err != nil {
			return errors.Wrapf(err, "error registering container name %s", ctr.Name())
		}
		if err := nsIndex.idIndex.Add(ctr.ID()); err != nil {
			return errors.Wrapf(err, "error registering container ID %s", ctr.ID())
		}
	}

	// Add containers this container depends on
	for _, depCtr := range depCtrs {
		s.addCtrToDependsMap(ctr.ID(), depCtr)
	}

	// Add container to volume dependencies
	for _, vol := range ctr.config.Spec.Mounts {
		if strings.Contains(vol.Source, ctr.runtime.config.VolumePath) {
			volName := strings.Split(vol.Source[len(ctr.runtime.config.VolumePath)+1:], "/")[0]
			s.addCtrToVolDependsMap(ctr.ID(), volName)
		}
	}

	return nil
}

// RemoveContainer removes a container from the state
// The container will only be removed from the state, not from the pod the container belongs to
func (s *InMemoryState) RemoveContainer(ctr *Container) error {
	// Almost no validity checks are performed, to ensure we can kick
	// misbehaving containers out of the state

	if err := s.checkNSMatch(ctr.ID(), ctr.Namespace()); err != nil {
		return err
	}

	// Ensure we don't remove a container which other containers depend on
	deps, ok := s.ctrDepends[ctr.ID()]
	if ok && len(deps) != 0 {
		depsStr := strings.Join(deps, ", ")
		return errors.Wrapf(ErrCtrExists, "the following containers depend on container %s: %s", ctr.ID(), depsStr)
	}

	if _, ok := s.containers[ctr.ID()]; !ok {
		ctr.valid = false
		return errors.Wrapf(ErrNoSuchCtr, "no container exists in state with ID %s", ctr.ID())
	}

	if err := s.idIndex.Delete(ctr.ID()); err != nil {
		return errors.Wrapf(err, "error removing container ID from index")
	}
	delete(s.containers, ctr.ID())
	s.nameIndex.Release(ctr.Name())

	delete(s.ctrDepends, ctr.ID())

	if ctr.config.Namespace != "" {
		nsIndex, ok := s.namespaceIndexes[ctr.config.Namespace]
		if !ok {
			return errors.Wrapf(ErrInternal, "error retrieving index for namespace %q", ctr.config.Namespace)
		}
		if err := nsIndex.idIndex.Delete(ctr.ID()); err != nil {
			return errors.Wrapf(err, "error removing container %s from namespace ID index", ctr.ID())
		}
		nsIndex.nameIndex.Release(ctr.Name())
	}

	// Remove us from container dependencies
	depCtrs := ctr.Dependencies()
	for _, depCtr := range depCtrs {
		s.removeCtrFromDependsMap(ctr.ID(), depCtr)
	}

	// Remove container from volume dependencies
	for _, vol := range ctr.config.Spec.Mounts {
		if strings.Contains(vol.Source, ctr.runtime.config.VolumePath) {
			volName := strings.Split(vol.Source[len(ctr.runtime.config.VolumePath)+1:], "/")[0]
			s.removeCtrFromVolDependsMap(ctr.ID(), volName)
		}
	}

	return nil
}

// UpdateContainer updates a container's state
// As all state is in-memory, no update will be required
// As such this is a no-op
func (s *InMemoryState) UpdateContainer(ctr *Container) error {
	// If the container is invalid, return error
	if !ctr.valid {
		return errors.Wrapf(ErrCtrRemoved, "container with ID %s is not valid", ctr.ID())
	}

	// If the container does not exist, return error
	if _, ok := s.containers[ctr.ID()]; !ok {
		ctr.valid = false
		return errors.Wrapf(ErrNoSuchCtr, "container with ID %s not found in state", ctr.ID())
	}

	return s.checkNSMatch(ctr.ID(), ctr.Namespace())
}

// SaveContainer saves a container's state
// As all state is in-memory, any changes are always reflected as soon as they
// are made
// As such this is a no-op
func (s *InMemoryState) SaveContainer(ctr *Container) error {
	// If the container is invalid, return error
	if !ctr.valid {
		return errors.Wrapf(ErrCtrRemoved, "container with ID %s is not valid", ctr.ID())
	}

	// If the container does not exist, return error
	if _, ok := s.containers[ctr.ID()]; !ok {
		ctr.valid = false
		return errors.Wrapf(ErrNoSuchCtr, "container with ID %s not found in state", ctr.ID())
	}

	return s.checkNSMatch(ctr.ID(), ctr.Namespace())
}

// ContainerInUse checks if the given container is being used by other containers
func (s *InMemoryState) ContainerInUse(ctr *Container) ([]string, error) {
	if !ctr.valid {
		return nil, ErrCtrRemoved
	}

	// If the container does not exist, return error
	if _, ok := s.containers[ctr.ID()]; !ok {
		ctr.valid = false
		return nil, errors.Wrapf(ErrNoSuchCtr, "container with ID %s not found in state", ctr.ID())
	}

	if err := s.checkNSMatch(ctr.ID(), ctr.Namespace()); err != nil {
		return nil, err
	}

	arr, ok := s.ctrDepends[ctr.ID()]
	if !ok {
		return []string{}, nil
	}

	return arr, nil
}

// Volume retrieves a volume from its full name
func (s *InMemoryState) Volume(name string) (*Volume, error) {
	if name == "" {
		return nil, ErrEmptyID
	}

	vol, ok := s.volumes[name]
	if !ok {
		return nil, errors.Wrapf(ErrNoSuchCtr, "no volume with name %s found", name)
	}

	return vol, nil
}

// HasVolume checks if a volume with the given name is present in the state
func (s *InMemoryState) HasVolume(name string) (bool, error) {
	if name == "" {
		return false, ErrEmptyID
	}

	_, ok := s.volumes[name]
	if !ok {
		return false, nil
	}

	return true, nil
}

// AddVolume adds a volume to the state
func (s *InMemoryState) AddVolume(volume *Volume) error {
	if !volume.valid {
		return errors.Wrapf(ErrVolumeRemoved, "volume with name %s is not valid", volume.Name())
	}

	if _, ok := s.volumes[volume.Name()]; ok {
		return errors.Wrapf(ErrVolumeExists, "volume with name %s already exists in state", volume.Name())
	}

	s.volumes[volume.Name()] = volume

	return nil
}

// RemoveVolume removes a volume from the state
func (s *InMemoryState) RemoveVolume(volume *Volume) error {
	// Ensure we don't remove a volume which containers depend on
	deps, ok := s.volumeDepends[volume.Name()]
	if ok && len(deps) != 0 {
		depsStr := strings.Join(deps, ", ")
		return errors.Wrapf(ErrVolumeExists, "the following containers depend on volume %s: %s", volume.Name(), depsStr)
	}

	if _, ok := s.volumes[volume.Name()]; !ok {
		volume.valid = false
		return errors.Wrapf(ErrVolumeRemoved, "no volume exists in state with name %s", volume.Name())
	}

	delete(s.volumes, volume.Name())

	return nil
}

// RemoveVolCtrDep updates the container dependencies of the volume
func (s *InMemoryState) RemoveVolCtrDep(volume *Volume, ctrID string) error {
	if !volume.valid {
		return errors.Wrapf(ErrVolumeRemoved, "volume with name %s is not valid", volume.Name())
	}

	if _, ok := s.volumes[volume.Name()]; !ok {
		return errors.Wrapf(ErrNoSuchVolume, "volume with name %s doesn't exists in state", volume.Name())
	}

	// Remove container that is using this volume
	s.removeCtrFromVolDependsMap(ctrID, volume.Name())

	return nil
}

// VolumeInUse checks if the given volume is being used by at least one container
func (s *InMemoryState) VolumeInUse(volume *Volume) ([]string, error) {
	if !volume.valid {
		return nil, ErrVolumeRemoved
	}

	// If the volume does not exist, return error
	if _, ok := s.volumes[volume.Name()]; !ok {
		volume.valid = false
		return nil, errors.Wrapf(ErrNoSuchVolume, "volume with name %s not found in state", volume.Name())
	}

	arr, ok := s.volumeDepends[volume.Name()]
	if !ok {
		return []string{}, nil
	}

	return arr, nil
}

// AllVolumes returns all volumes that exist in the state
func (s *InMemoryState) AllVolumes() ([]*Volume, error) {
	allVols := make([]*Volume, 0, len(s.volumes))
	for _, v := range s.volumes {
		allVols = append(allVols, v)
	}

	return allVols, nil
}

// AllContainers retrieves all containers from the state
func (s *InMemoryState) AllContainers() ([]*Container, error) {
	ctrs := make([]*Container, 0, len(s.containers))
	for _, ctr := range s.containers {
		if s.namespace == "" || ctr.config.Namespace == s.namespace {
			ctrs = append(ctrs, ctr)
		}
	}

	return ctrs, nil
}

// Pod retrieves a pod from the state from its full ID
func (s *InMemoryState) Pod(id string) (*Pod, error) {
	if id == "" {
		return nil, ErrEmptyID
	}

	pod, ok := s.pods[id]
	if !ok {
		return nil, errors.Wrapf(ErrNoSuchPod, "no pod with id %s found", id)
	}

	if err := s.checkNSMatch(pod.ID(), pod.Namespace()); err != nil {
		return nil, err
	}

	return pod, nil
}

// LookupPod retrieves a pod from the state from a full or unique partial ID or
// a full name
func (s *InMemoryState) LookupPod(idOrName string) (*Pod, error) {
	var (
		nameIndex *registrar.Registrar
		idIndex   *truncindex.TruncIndex
	)

	if idOrName == "" {
		return nil, ErrEmptyID
	}

	if s.namespace != "" {
		nsIndex, ok := s.namespaceIndexes[s.namespace]
		if !ok {
			// We have no containers in the namespace
			// Return false
			return nil, errors.Wrapf(ErrNoSuchCtr, "no container found with name or ID %s", idOrName)
		}
		nameIndex = nsIndex.nameIndex
		idIndex = nsIndex.idIndex
	} else {
		nameIndex = s.nameIndex
		idIndex = s.idIndex
	}

	fullID, err := nameIndex.Get(idOrName)
	if err != nil {
		if err == registrar.ErrNameNotReserved {
			// What was passed is not a name, assume it's an ID
			fullID, err = idIndex.Get(idOrName)
			if err != nil {
				if err == truncindex.ErrNotExist {
					return nil, errors.Wrapf(ErrNoSuchPod, "no pod found with name or ID %s", idOrName)
				}
				return nil, errors.Wrapf(err, "error performing truncindex lookup for ID %s", idOrName)
			}
		} else {
			return nil, errors.Wrapf(err, "error performing registry lookup for ID %s", idOrName)
		}
	}

	pod, ok := s.pods[fullID]
	if !ok {
		// It's a container not a pod
		return nil, errors.Wrapf(ErrNoSuchPod, "id or name %s is a container not a pod", idOrName)
	}

	return pod, nil
}

// HasPod checks if a pod with the given ID is present in the state
func (s *InMemoryState) HasPod(id string) (bool, error) {
	if id == "" {
		return false, ErrEmptyID
	}

	pod, ok := s.pods[id]
	if !ok || (s.namespace != "" && s.namespace != pod.config.Namespace) {
		return false, nil
	}

	return true, nil
}

// PodHasContainer checks if the given pod has a container with the given ID
func (s *InMemoryState) PodHasContainer(pod *Pod, ctrID string) (bool, error) {
	if !pod.valid {
		return false, errors.Wrapf(ErrPodRemoved, "pod %s is not valid", pod.ID())
	}

	if ctrID == "" {
		return false, ErrEmptyID
	}

	if err := s.checkNSMatch(pod.ID(), pod.Namespace()); err != nil {
		return false, err
	}

	podCtrs, ok := s.podContainers[pod.ID()]
	if !ok {
		pod.valid = false
		return false, errors.Wrapf(ErrNoSuchPod, "no pod with ID %s found in state", pod.ID())
	}

	_, ok = podCtrs[ctrID]
	return ok, nil
}

// PodContainersByID returns the IDs of all containers in the given pod
func (s *InMemoryState) PodContainersByID(pod *Pod) ([]string, error) {
	if !pod.valid {
		return nil, errors.Wrapf(ErrPodRemoved, "pod %s is not valid", pod.ID())
	}

	if err := s.checkNSMatch(pod.ID(), pod.Namespace()); err != nil {
		return nil, err
	}

	podCtrs, ok := s.podContainers[pod.ID()]
	if !ok {
		pod.valid = false
		return nil, errors.Wrapf(ErrNoSuchPod, "no pod with ID %s found in state", pod.ID())
	}

	length := len(podCtrs)
	if length == 0 {
		return []string{}, nil
	}

	ctrs := make([]string, 0, length)
	for _, ctr := range podCtrs {
		ctrs = append(ctrs, ctr.ID())
	}

	return ctrs, nil
}

// PodContainers retrieves the containers from a pod
func (s *InMemoryState) PodContainers(pod *Pod) ([]*Container, error) {
	if !pod.valid {
		return nil, errors.Wrapf(ErrPodRemoved, "pod %s is not valid", pod.ID())
	}

	if err := s.checkNSMatch(pod.ID(), pod.Namespace()); err != nil {
		return nil, err
	}

	podCtrs, ok := s.podContainers[pod.ID()]
	if !ok {
		pod.valid = false
		return nil, errors.Wrapf(ErrNoSuchPod, "no pod with ID %s found in state", pod.ID())
	}

	length := len(podCtrs)
	if length == 0 {
		return []*Container{}, nil
	}

	ctrs := make([]*Container, 0, length)
	for _, ctr := range podCtrs {
		ctrs = append(ctrs, ctr)
	}

	return ctrs, nil
}

// AddPod adds a given pod to the state
func (s *InMemoryState) AddPod(pod *Pod) error {
	if !pod.valid {
		return errors.Wrapf(ErrPodRemoved, "pod %s is not valid and cannot be added", pod.ID())
	}

	if err := s.checkNSMatch(pod.ID(), pod.Namespace()); err != nil {
		return err
	}

	if _, ok := s.pods[pod.ID()]; ok {
		return errors.Wrapf(ErrPodExists, "pod with ID %s already exists in state", pod.ID())
	}

	if _, ok := s.podContainers[pod.ID()]; ok {
		return errors.Wrapf(ErrPodExists, "pod with ID %s already exists in state", pod.ID())
	}

	if err := s.nameIndex.Reserve(pod.Name(), pod.ID()); err != nil {
		return errors.Wrapf(err, "error registering pod name %s", pod.Name())
	}

	if err := s.idIndex.Add(pod.ID()); err != nil {
		s.nameIndex.Release(pod.Name())
		return errors.Wrapf(err, "error registering pod ID %s", pod.ID())
	}

	s.pods[pod.ID()] = pod

	s.podContainers[pod.ID()] = make(map[string]*Container)

	// If we're in a namespace, add us to that namespace's indexes
	if pod.config.Namespace != "" {
		var nsIndex *namespaceIndex
		nsIndex, ok := s.namespaceIndexes[pod.config.Namespace]
		if !ok {
			nsIndex = new(namespaceIndex)
			nsIndex.nameIndex = registrar.NewRegistrar()
			nsIndex.idIndex = truncindex.NewTruncIndex([]string{})
			s.namespaceIndexes[pod.config.Namespace] = nsIndex
		}
		// Should be no errors here, the previous index adds should have caught that
		if err := nsIndex.nameIndex.Reserve(pod.Name(), pod.ID()); err != nil {
			return errors.Wrapf(err, "error registering container name %s", pod.Name())
		}
		if err := nsIndex.idIndex.Add(pod.ID()); err != nil {
			return errors.Wrapf(err, "error registering container ID %s", pod.ID())
		}
	}

	return nil
}

// RemovePod removes a given pod from the state
// Only empty pods can be removed
func (s *InMemoryState) RemovePod(pod *Pod) error {
	// Don't make many validity checks to ensure we can kick badly formed
	// pods out of the state

	if err := s.checkNSMatch(pod.ID(), pod.Namespace()); err != nil {
		return err
	}

	if _, ok := s.pods[pod.ID()]; !ok {
		pod.valid = false
		return errors.Wrapf(ErrNoSuchPod, "no pod exists in state with ID %s", pod.ID())
	}
	podCtrs, ok := s.podContainers[pod.ID()]
	if !ok {
		pod.valid = false
		return errors.Wrapf(ErrNoSuchPod, "no pod exists in state with ID %s", pod.ID())
	}
	if len(podCtrs) != 0 {
		return errors.Wrapf(ErrCtrExists, "pod %s is not empty and cannot be removed", pod.ID())
	}

	if err := s.idIndex.Delete(pod.ID()); err != nil {
		return errors.Wrapf(err, "error removing pod ID %s from index", pod.ID())
	}
	delete(s.pods, pod.ID())
	delete(s.podContainers, pod.ID())
	s.nameIndex.Release(pod.Name())

	if pod.config.Namespace != "" {
		nsIndex, ok := s.namespaceIndexes[pod.config.Namespace]
		if !ok {
			return errors.Wrapf(ErrInternal, "error retrieving index for namespace %q", pod.config.Namespace)
		}
		if err := nsIndex.idIndex.Delete(pod.ID()); err != nil {
			return errors.Wrapf(err, "error removing container %s from namespace ID index", pod.ID())
		}
		nsIndex.nameIndex.Release(pod.Name())
	}

	return nil
}

// RemovePodContainers removes all containers from a pod
// This is used to simultaneously remove a number of containers with
// many interdependencies
// Will only remove containers if no dependencies outside of the pod are present
func (s *InMemoryState) RemovePodContainers(pod *Pod) error {
	if !pod.valid {
		return errors.Wrapf(ErrPodRemoved, "pod %s is not valid", pod.ID())
	}

	if err := s.checkNSMatch(pod.ID(), pod.Namespace()); err != nil {
		return err
	}

	// Get pod containers
	podCtrs, ok := s.podContainers[pod.ID()]
	if !ok {
		pod.valid = false
		return errors.Wrapf(ErrNoSuchPod, "no pod exists in state with ID %s", pod.ID())
	}

	// Go through container dependencies. Check to see if any are outside the pod.
	for ctr := range podCtrs {
		ctrDeps, ok := s.ctrDepends[ctr]
		if ok {
			for _, dep := range ctrDeps {
				if _, ok := podCtrs[dep]; !ok {
					return errors.Wrapf(ErrCtrExists, "container %s has dependency %s outside of pod %s", ctr, dep, pod.ID())
				}
			}
		}
	}

	// All dependencies are OK to remove
	// Remove all containers
	s.podContainers[pod.ID()] = make(map[string]*Container)
	for _, ctr := range podCtrs {
		if err := s.idIndex.Delete(ctr.ID()); err != nil {
			return errors.Wrapf(err, "error removing container ID from index")
		}
		s.nameIndex.Release(ctr.Name())

		delete(s.containers, ctr.ID())
		delete(s.ctrDepends, ctr.ID())
	}

	return nil
}

// AddContainerToPod adds a container to the given pod, also adding it to the
// state
func (s *InMemoryState) AddContainerToPod(pod *Pod, ctr *Container) error {
	if !pod.valid {
		return errors.Wrapf(ErrPodRemoved, "pod %s is not valid", pod.ID())
	}
	if !ctr.valid {
		return errors.Wrapf(ErrCtrRemoved, "container %s is not valid", ctr.ID())
	}

	if ctr.config.Pod != pod.ID() {
		return errors.Wrapf(ErrInvalidArg, "container %s is not in pod %s", ctr.ID(), pod.ID())
	}

	if ctr.config.Namespace != pod.config.Namespace {
		return errors.Wrapf(ErrNSMismatch, "container %s is in namespace %s and pod %s is in namespace %s",
			ctr.ID(), ctr.config.Namespace, pod.ID(), pod.config.Namespace)
	}

	if err := s.checkNSMatch(ctr.ID(), ctr.Namespace()); err != nil {
		return err
	}

	// Retrieve pod containers list
	podCtrs, ok := s.podContainers[pod.ID()]
	if !ok {
		pod.valid = false
		return errors.Wrapf(ErrPodRemoved, "pod %s not found in state", pod.ID())
	}

	// Is the container already in the pod?
	if _, ok = podCtrs[ctr.ID()]; ok {
		return errors.Wrapf(ErrCtrExists, "container with ID %s already exists in pod %s", ctr.ID(), pod.ID())
	}

	// There are potential race conditions with this
	// But in-memory state is intended purely for testing and not production
	// use, so this should be fine.
	depCtrs := ctr.Dependencies()
	for _, depCtr := range depCtrs {
		if _, ok = s.containers[depCtr]; !ok {
			return errors.Wrapf(ErrNoSuchCtr, "cannot depend on nonexistent container %s", depCtr)
		}
		depCtrStruct, ok := podCtrs[depCtr]
		if !ok {
			return errors.Wrapf(ErrInvalidArg, "cannot depend on container %s as it is not in pod %s", depCtr, pod.ID())
		}
		if depCtrStruct.config.Namespace != ctr.config.Namespace {
			return errors.Wrapf(ErrNSMismatch, "container %s is in namespace %s and cannot depend on container %s in namespace %s", ctr.ID(), ctr.config.Namespace, depCtr, depCtrStruct.config.Namespace)
		}
	}

	// Add container to state
	if _, ok = s.containers[ctr.ID()]; ok {
		return errors.Wrapf(ErrCtrExists, "container with ID %s already exists in state", ctr.ID())
	}

	if err := s.nameIndex.Reserve(ctr.Name(), ctr.ID()); err != nil {
		return errors.Wrapf(err, "error reserving container name %s", ctr.Name())
	}

	if err := s.idIndex.Add(ctr.ID()); err != nil {
		s.nameIndex.Release(ctr.Name())
		return errors.Wrapf(err, "error releasing container ID %s", ctr.ID())
	}

	s.containers[ctr.ID()] = ctr

	// Add container to pod containers
	podCtrs[ctr.ID()] = ctr

	// If we're in a namespace, add us to that namespace's indexes
	if ctr.config.Namespace != "" {
		var nsIndex *namespaceIndex
		nsIndex, ok := s.namespaceIndexes[ctr.config.Namespace]
		if !ok {
			nsIndex = new(namespaceIndex)
			nsIndex.nameIndex = registrar.NewRegistrar()
			nsIndex.idIndex = truncindex.NewTruncIndex([]string{})
			s.namespaceIndexes[ctr.config.Namespace] = nsIndex
		}
		// Should be no errors here, the previous index adds should have caught that
		if err := nsIndex.nameIndex.Reserve(ctr.Name(), ctr.ID()); err != nil {
			return errors.Wrapf(err, "error registering container name %s", ctr.Name())
		}
		if err := nsIndex.idIndex.Add(ctr.ID()); err != nil {
			return errors.Wrapf(err, "error registering container ID %s", ctr.ID())
		}
	}

	// Add containers this container depends on
	for _, depCtr := range depCtrs {
		s.addCtrToDependsMap(ctr.ID(), depCtr)
	}

	return nil
}

// RemoveContainerFromPod removes the given container from the given pod
// The container is also removed from the state
func (s *InMemoryState) RemoveContainerFromPod(pod *Pod, ctr *Container) error {
	if !pod.valid {
		return errors.Wrapf(ErrPodRemoved, "pod %s is not valid and containers cannot be removed", pod.ID())
	}
	if !ctr.valid {
		return errors.Wrapf(ErrCtrRemoved, "container %s is not valid and cannot be removed from the pod", ctr.ID())
	}

	if err := s.checkNSMatch(ctr.ID(), ctr.Namespace()); err != nil {
		return err
	}

	// Ensure we don't remove a container which other containers depend on
	deps, ok := s.ctrDepends[ctr.ID()]
	if ok && len(deps) != 0 {
		depsStr := strings.Join(deps, ", ")
		return errors.Wrapf(ErrCtrExists, "the following containers depend on container %s: %s", ctr.ID(), depsStr)
	}

	// Retrieve pod containers
	podCtrs, ok := s.podContainers[pod.ID()]
	if !ok {
		pod.valid = false
		return errors.Wrapf(ErrPodRemoved, "pod %s has been removed", pod.ID())
	}

	// Does the container exist?
	if _, ok := s.containers[ctr.ID()]; !ok {
		ctr.valid = false
		return errors.Wrapf(ErrNoSuchCtr, "container %s does not exist in state", ctr.ID())
	}

	// Is the container in the pod?
	if _, ok := podCtrs[ctr.ID()]; !ok {
		return errors.Wrapf(ErrNoSuchCtr, "container with ID %s not found in pod %s", ctr.ID(), pod.ID())
	}

	// Remove container from state
	if _, ok := s.containers[ctr.ID()]; !ok {
		return errors.Wrapf(ErrNoSuchCtr, "no container exists in state with ID %s", ctr.ID())
	}

	if err := s.idIndex.Delete(ctr.ID()); err != nil {
		return errors.Wrapf(err, "error removing container ID from index")
	}
	delete(s.containers, ctr.ID())
	s.nameIndex.Release(ctr.Name())

	// Remove the container from the pod
	delete(podCtrs, ctr.ID())

	if ctr.config.Namespace != "" {
		nsIndex, ok := s.namespaceIndexes[ctr.config.Namespace]
		if !ok {
			return errors.Wrapf(ErrInternal, "error retrieving index for namespace %q", ctr.config.Namespace)
		}
		if err := nsIndex.idIndex.Delete(ctr.ID()); err != nil {
			return errors.Wrapf(err, "error removing container %s from namespace ID index", ctr.ID())
		}
		nsIndex.nameIndex.Release(ctr.Name())
	}

	// Remove us from container dependencies
	depCtrs := ctr.Dependencies()
	for _, depCtr := range depCtrs {
		s.removeCtrFromDependsMap(ctr.ID(), depCtr)
	}

	return nil
}

// UpdatePod updates a pod in the state
// This is a no-op as there is no backing store
func (s *InMemoryState) UpdatePod(pod *Pod) error {
	if !pod.valid {
		return ErrPodRemoved
	}

	if err := s.checkNSMatch(pod.ID(), pod.Namespace()); err != nil {
		return err
	}

	if _, ok := s.pods[pod.ID()]; !ok {
		pod.valid = false
		return errors.Wrapf(ErrNoSuchPod, "no pod exists in state with ID %s", pod.ID())
	}

	return nil
}

// SavePod updates a pod in the state
// This is a no-op at there is no backing store
func (s *InMemoryState) SavePod(pod *Pod) error {
	if !pod.valid {
		return ErrPodRemoved
	}

	if err := s.checkNSMatch(pod.ID(), pod.Namespace()); err != nil {
		return err
	}

	if _, ok := s.pods[pod.ID()]; !ok {
		pod.valid = false
		return errors.Wrapf(ErrNoSuchPod, "no pod exists in state with ID %s", pod.ID())
	}

	return nil
}

// AllPods retrieves all pods currently in the state
func (s *InMemoryState) AllPods() ([]*Pod, error) {
	pods := make([]*Pod, 0, len(s.pods))
	for _, pod := range s.pods {
		if s.namespace != "" {
			if s.namespace == pod.config.Namespace {
				pods = append(pods, pod)
			}
		} else {
			pods = append(pods, pod)
		}
	}

	return pods, nil
}

// Internal Functions

// Add a container to the dependency mappings
func (s *InMemoryState) addCtrToDependsMap(ctrID, dependsID string) {
	if dependsID != "" {
		arr, ok := s.ctrDepends[dependsID]
		if !ok {
			// Do not have a mapping for that container yet
			s.ctrDepends[dependsID] = []string{ctrID}
		} else {
			// Have a mapping for the container
			arr = append(arr, ctrID)
			s.ctrDepends[dependsID] = arr
		}
	}
}

// Remove a container from dependency mappings
func (s *InMemoryState) removeCtrFromDependsMap(ctrID, dependsID string) {
	if dependsID != "" {
		arr, ok := s.ctrDepends[dependsID]
		if !ok {
			// Internal state seems inconsistent
			// But the dependency is definitely gone
			// So just return
			return
		}

		newArr := make([]string, 0, len(arr))

		for _, id := range arr {
			if id != ctrID {
				newArr = append(newArr, id)
			}
		}

		s.ctrDepends[dependsID] = newArr
	}
}

// Add a container to the dependency mappings for the volume
func (s *InMemoryState) addCtrToVolDependsMap(depCtrID, volName string) {
	if volName != "" {
		arr, ok := s.volumeDepends[volName]
		if !ok {
			// Do not have a mapping for that volume yet
			s.volumeDepends[volName] = []string{depCtrID}
		} else {
			// Have a mapping for the volume
			arr = append(arr, depCtrID)
			s.volumeDepends[volName] = arr
		}
	}
}

// Remove a container from the dependency mappings for the volume
func (s *InMemoryState) removeCtrFromVolDependsMap(depCtrID, volName string) {
	if volName != "" {
		arr, ok := s.volumeDepends[volName]
		if !ok {
			// Internal state seems inconsistent
			// But the dependency is definitely gone
			// So just return
			return
		}

		newArr := make([]string, 0, len(arr))

		for _, id := range arr {
			if id != depCtrID {
				newArr = append(newArr, id)
			}
		}

		s.volumeDepends[volName] = newArr
	}
}

// Check if we can access a pod or container, or if that is blocked by
// namespaces.
func (s *InMemoryState) checkNSMatch(id, ns string) error {
	if s.namespace != "" && s.namespace != ns {
		return errors.Wrapf(ErrNSMismatch, "cannot access %s as it is in namespace %q and we are in namespace %q",
			id, ns, s.namespace)
	}
	return nil
}
