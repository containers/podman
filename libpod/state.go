package libpod

// State is a storage backend for libpod's current state
type State interface {
	// Close performs any pre-exit cleanup (e.g. closing database
	// connections) that may be required
	Close() error

	// Refresh clears container and pod states after a reboot
	Refresh() error

	// Accepts full ID of container
	Container(id string) (*Container, error)
	// Accepts full or partial IDs (as long as they are unique) and names
	LookupContainer(idOrName string) (*Container, error)
	// Checks if a container with the given ID is present in the state
	HasContainer(id string) (bool, error)
	// Adds container to state
	// The container cannot be part of a pod
	AddContainer(ctr *Container) error
	// Removes container from state
	// The container will only be removed from the state, not from the pod
	// which the container belongs to
	RemoveContainer(ctr *Container) error
	// UpdateContainer updates a container's state from the backing store
	UpdateContainer(ctr *Container) error
	// SaveContainer saves a container's current state to the backing store
	SaveContainer(ctr *Container) error
	// ContainerInUse checks if other containers depend upon a given
	// container
	// It returns a slice of the IDs of containers which depend on the given
	// container. If the slice is empty, no container depend on the given
	// container.
	// A container cannot be removed if other containers depend on it
	ContainerInUse(ctr *Container) ([]string, error)
	// Retrieves all containers presently in state
	AllContainers() ([]*Container, error)

	// Accepts full ID of pod
	Pod(id string) (*Pod, error)
	// Accepts full or partial IDs (as long as they are unique) and names
	LookupPod(idOrName string) (*Pod, error)
	// Checks if a pod with the given ID is present in the state
	HasPod(id string) (bool, error)
	// Check if a pod has a container with the given ID
	PodHasContainer(pod *Pod, ctrID string) (bool, error)
	// Get the IDs of all containers in a pod
	PodContainersByID(pod *Pod) ([]string, error)
	// Get all the containers in a pod
	PodContainers(pod *Pod) ([]*Container, error)
	// Adds pod to state
	AddPod(pod *Pod) error
	// Removes pod from state
	// Only empty pods can be removed from the state
	RemovePod(pod *Pod) error
	// Remove all containers from a pod
	// Used to simulataneously remove containers that might otherwise have
	// dependency issues
	// Will fail if a dependency outside the pod is encountered
	RemovePodContainers(pod *Pod) error
	// AddContainerToPod adds a container to an existing pod
	// The container given will be added to the state and the pod
	AddContainerToPod(pod *Pod, ctr *Container) error
	// RemoveContainerFromPod removes a container from an existing pod
	// The container will also be removed from the state
	RemoveContainerFromPod(pod *Pod, ctr *Container) error
	// Retrieves all pods presently in state
	AllPods() ([]*Pod, error)
}
