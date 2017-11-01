package libpod

import (
	"github.com/docker/docker/pkg/truncindex"
	"github.com/kubernetes-incubator/cri-o/pkg/registrar"
	"github.com/pkg/errors"
)

// An InMemoryState is a purely in-memory state store
type InMemoryState struct {
	pods         map[string]*Pod
	containers   map[string]*Container
	podNameIndex *registrar.Registrar
	podIDIndex   *truncindex.TruncIndex
	ctrNameIndex *registrar.Registrar
	ctrIDIndex   *truncindex.TruncIndex
}

// NewInMemoryState initializes a new, empty in-memory state
func NewInMemoryState() (State, error) {
	state := new(InMemoryState)

	state.pods = make(map[string]*Pod)
	state.containers = make(map[string]*Container)

	state.podNameIndex = registrar.NewRegistrar()
	state.ctrNameIndex = registrar.NewRegistrar()

	state.podIDIndex = truncindex.NewTruncIndex([]string{})
	state.ctrIDIndex = truncindex.NewTruncIndex([]string{})

	return state, nil
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

	return ctr, nil
}

// LookupContainer retrieves a container by full ID, unique partial ID, or name
func (s *InMemoryState) LookupContainer(idOrName string) (*Container, error) {
	if idOrName == "" {
		return nil, ErrEmptyID
	}

	fullID, err := s.ctrNameIndex.Get(idOrName)
	if err != nil {
		if err == registrar.ErrNameNotReserved {
			// What was passed is not a name, assume it's an ID
			fullID, err = s.ctrIDIndex.Get(idOrName)
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
		// This should never happen
		return nil, errors.Wrapf(ErrInternal, "mismatch in container ID registry and containers map for ID %s", fullID)
	}

	return ctr, nil
}

// HasContainer checks if a container with the given ID is present in the state
func (s *InMemoryState) HasContainer(id string) (bool, error) {
	if id == "" {
		return false, ErrEmptyID
	}

	_, ok := s.containers[id]

	return ok, nil
}

// AddContainer adds a container to the state
// If the container belongs to a pod, the pod must already be present when the
// container is added, and the container must be present in the pod
func (s *InMemoryState) AddContainer(ctr *Container) error {
	if !ctr.valid {
		return errors.Wrapf(ErrCtrRemoved, "container with ID %s is not valid", ctr.ID())
	}

	_, ok := s.containers[ctr.ID()]
	if ok {
		return errors.Wrapf(ErrCtrExists, "container with ID %s already exists in state", ctr.ID())
	}

	if ctr.pod != nil {
		if _, ok := s.pods[ctr.pod.ID()]; !ok {
			return errors.Wrapf(ErrNoSuchPod, "pod %s does not exist, cannot add container %s", ctr.pod.ID(), ctr.ID())
		}

		hasCtr, err := ctr.pod.HasContainer(ctr.ID())
		if err != nil {
			return errors.Wrapf(err, "error checking if container %s is present in pod %s", ctr.ID(), ctr.pod.ID())
		} else if !hasCtr {
			return errors.Wrapf(ErrNoSuchCtr, "container %s is not present in pod %s", ctr.ID(), ctr.pod.ID())
		}
	}

	if err := s.ctrNameIndex.Reserve(ctr.Name(), ctr.ID()); err != nil {
		return errors.Wrapf(err, "error registering container name %s", ctr.Name())
	}

	if err := s.ctrIDIndex.Add(ctr.ID()); err != nil {
		s.ctrNameIndex.Release(ctr.Name())
		return errors.Wrapf(err, "error registering container ID %s", ctr.ID())
	}

	s.containers[ctr.ID()] = ctr

	return nil
}

// RemoveContainer removes a container from the state
// The container will only be removed from the state, not from the pod the container belongs to
func (s *InMemoryState) RemoveContainer(ctr *Container) error {
	// Almost no validity checks are performed, to ensure we can kick
	// misbehaving containers out of the state

	if _, ok := s.containers[ctr.ID()]; !ok {
		return errors.Wrapf(ErrNoSuchCtr, "no container exists in state with ID %s", ctr.ID())
	}

	if err := s.ctrIDIndex.Delete(ctr.ID()); err != nil {
		return errors.Wrapf(err, "error removing container ID from index")
	}
	delete(s.containers, ctr.ID())
	s.ctrNameIndex.Release(ctr.Name())

	return nil
}

// AllContainers retrieves all containers from the state
func (s *InMemoryState) AllContainers() ([]*Container, error) {
	ctrs := make([]*Container, 0, len(s.containers))
	for _, ctr := range s.containers {
		ctrs = append(ctrs, ctr)
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

	return pod, nil
}

// LookupPod retrieves a pod from the state from a full or unique partial ID or
// a full name
func (s *InMemoryState) LookupPod(idOrName string) (*Pod, error) {
	if idOrName == "" {
		return nil, ErrEmptyID
	}

	fullID, err := s.podNameIndex.Get(idOrName)
	if err != nil {
		if err == registrar.ErrNameNotReserved {
			// What was passed is not a name, assume it's an ID
			fullID, err = s.podIDIndex.Get(idOrName)
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
		// This should never happen
		return nil, errors.Wrapf(ErrInternal, "mismatch in pod ID registry and pod map for ID %s", fullID)
	}

	return pod, nil
}

// HasPod checks if a pod with the given ID is present in the state
func (s *InMemoryState) HasPod(id string) (bool, error) {
	if id == "" {
		return false, ErrEmptyID
	}

	_, ok := s.pods[id]

	return ok, nil
}

// AddPod adds a given pod to the state
// Only empty pods can be added to the state
func (s *InMemoryState) AddPod(pod *Pod) error {
	if !pod.valid {
		return errors.Wrapf(ErrPodRemoved, "pod %s is not valid and cannot be added", pod.ID())
	}

	if _, ok := s.pods[pod.ID()]; ok {
		return errors.Wrapf(ErrPodExists, "pod with ID %s already exists in state", pod.ID())
	}

	if len(pod.containers) != 0 {
		return errors.Wrapf(ErrInternal, "only empty pods can be added to the state")
	}

	if err := s.podNameIndex.Reserve(pod.Name(), pod.ID()); err != nil {
		return errors.Wrapf(err, "error registering pod name %s", pod.Name())
	}

	if err := s.podIDIndex.Add(pod.ID()); err != nil {
		s.podNameIndex.Release(pod.Name())
		return errors.Wrapf(err, "error registering pod ID %s", pod.ID())
	}

	s.pods[pod.ID()] = pod

	return nil
}

// RemovePod removes a given pod from the state
// Containers within the pod will not be removed or changed
func (s *InMemoryState) RemovePod(pod *Pod) error {
	// Don't make many validity checks to ensure we can kick badly formed
	// pods out of the state

	if _, ok := s.pods[pod.ID()]; !ok {
		return errors.Wrapf(ErrNoSuchPod, "no pod exists in state with ID %s", pod.ID())
	}

	if err := s.podIDIndex.Delete(pod.ID()); err != nil {
		return errors.Wrapf(err, "error removing pod ID %s from index", pod.ID())
	}
	delete(s.pods, pod.ID())
	s.podNameIndex.Release(pod.Name())

	return nil
}

// AllPods retrieves all pods currently in the state
func (s *InMemoryState) AllPods() ([]*Pod, error) {
	pods := make([]*Pod, 0, len(s.pods))
	for _, pod := range s.pods {
		pods = append(pods, pod)
	}

	return pods, nil
}
