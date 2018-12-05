package libpod

// DBConfig is a set of Libpod runtime configuration settings that are saved
// in a State when it is first created, and can subsequently be retrieved.
type DBConfig struct {
	LibpodRoot  string
	LibpodTmp   string
	StorageRoot string
	StorageTmp  string
	GraphDriver string
}

// State is a storage backend for libpod's current state.
// A State is only initialized once per instance of libpod.
// As such, initialization methods for State implementations may safely assume
// they will be run as a singleton.
// For all container and pod retrieval methods, a State must retrieve the
// Configuration struct of the container or pod and include it in the returned
// struct. The State of the container or pod may optionally be included as well,
// but this is not a requirement.
// As such, all containers and pods must be synced with the database via the
// UpdateContainer and UpdatePod calls before any state-specific information is
// retrieved after they are pulled from the database.
// Generally speaking, the syncContainer() call should be run at the beginning
// of all API operations, which will silently handle this.
type State interface {
	// Close performs any pre-exit cleanup (e.g. closing database
	// connections) that may be required
	Close() error

	// Refresh clears container and pod states after a reboot
	Refresh() error

	// GetDBConfig retrieves several paths configured within the database
	// when it was created - namely, Libpod root and tmp dirs, c/storage
	// root and tmp dirs, and c/storage graph driver.
	// This is not implemented by the in-memory state, as it has no need to
	// validate runtime configuration.
	GetDBConfig() (*DBConfig, error)

	// ValidateDBConfig validates the config in the given Runtime struct
	// against paths stored in the configured database.
	// Libpod root and tmp dirs and c/storage root and tmp dirs and graph
	// driver are validated.
	// This is not implemented by the in-memory state, as it has no need to
	// validate runtime configuration that may change over multiple runs of
	// the program.
	ValidateDBConfig(runtime *Runtime) error

	// SetNamespace() sets the namespace for the store, and will determine
	// what containers are retrieved with container and pod retrieval calls.
	// A namespace of "", the empty string, acts as no namespace, and
	// containers and pods in all namespaces will be returned.
	SetNamespace(ns string) error

	// Return a container from the database from its full ID.
	// If the container is not in the set namespace, an error will be
	// returned.
	Container(id string) (*Container, error)
	// Return a container from the database by full or partial ID or full
	// name.
	// Containers not in the set namespace will be ignored.
	LookupContainer(idOrName string) (*Container, error)
	// Check if a container with the given full ID exists in the database.
	// If the container exists but is not in the set namespace, false will
	// be returned.
	HasContainer(id string) (bool, error)
	// Adds container to state.
	// The container cannot be part of a pod.
	// The container must have globally unique name and ID - pod names and
	// IDs also conflict with container names and IDs.
	// The container must be in the set namespace if a namespace has been
	// set.
	// All containers this container depends on must be part of the same
	// namespace and must not be joined to a pod.
	AddContainer(ctr *Container) error
	// Removes container from state.
	// Containers that are part of pods must use RemoveContainerFromPod.
	// The container must be part of the set namespace.
	RemoveContainer(ctr *Container) error
	// UpdateContainer updates a container's state from the backing store.
	// The container must be part of the set namespace.
	UpdateContainer(ctr *Container) error
	// SaveContainer saves a container's current state to the backing store.
	// The container must be part of the set namespace.
	SaveContainer(ctr *Container) error
	// ContainerInUse checks if other containers depend upon a given
	// container.
	// It returns a slice of the IDs of containers which depend on the given
	// container. If the slice is empty, no container depend on the given
	// container.
	// A container cannot be removed if other containers depend on it.
	// The container being checked must be part of the set namespace.
	ContainerInUse(ctr *Container) ([]string, error)
	// Retrieves all containers presently in state.
	// If a namespace is set, only containers within the namespace will be
	// returned.
	AllContainers() ([]*Container, error)

	// Accepts full ID of pod.
	// If the pod given is not in the set namespace, an error will be
	// returned.
	Pod(id string) (*Pod, error)
	// Accepts full or partial IDs (as long as they are unique) and names.
	// Pods not in the set namespace are ignored.
	LookupPod(idOrName string) (*Pod, error)
	// Checks if a pod with the given ID is present in the state.
	// If the given pod is not in the set namespace, false is returned.
	HasPod(id string) (bool, error)
	// Check if a pod has a container with the given ID.
	// The pod must be part of the set namespace.
	PodHasContainer(pod *Pod, ctrID string) (bool, error)
	// Get the IDs of all containers in a pod.
	// The pod must be part of the set namespace.
	PodContainersByID(pod *Pod) ([]string, error)
	// Get all the containers in a pod.
	// The pod must be part of the set namespace.
	PodContainers(pod *Pod) ([]*Container, error)
	// Adds pod to state.
	// The pod must be part of the set namespace.
	// The pod's name and ID must be globally unique.
	AddPod(pod *Pod) error
	// Removes pod from state.
	// Only empty pods can be removed from the state.
	// The pod must be part of the set namespace.
	RemovePod(pod *Pod) error
	// Remove all containers from a pod.
	// Used to simultaneously remove containers that might otherwise have
	// dependency issues.
	// Will fail if a dependency outside the pod is encountered.
	// The pod must be part of the set namespace.
	RemovePodContainers(pod *Pod) error
	// AddContainerToPod adds a container to an existing pod.
	// The container given will be added to the state and the pod.
	// The container and its dependencies must be part of the given pod,
	// and the given pod's namespace.
	// The pod must be part of the set namespace.
	// The pod must already exist in the state.
	// The container's name and ID must be globally unique.
	AddContainerToPod(pod *Pod, ctr *Container) error
	// RemoveContainerFromPod removes a container from an existing pod.
	// The container will also be removed from the state.
	// The container must be in the given pod, and the pod must be in the
	// set namespace.
	RemoveContainerFromPod(pod *Pod, ctr *Container) error
	// UpdatePod updates a pod's state from the database.
	// The pod must be in the set namespace.
	UpdatePod(pod *Pod) error
	// SavePod saves a pod's state to the database.
	// The pod must be in the set namespace.
	SavePod(pod *Pod) error
	// Retrieves all pods presently in state.
	// If a namespace has been set, only pods in that namespace will be
	// returned.
	AllPods() ([]*Pod, error)
}
