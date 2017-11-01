package apparmor

const (
	// DefaultApparmorProfile is the name of default apparmor profile name.
	DefaultApparmorProfile = "crio-default"

	// ContainerAnnotationKeyPrefix is the prefix to an annotation key specifying a container profile.
	ContainerAnnotationKeyPrefix = "container.apparmor.security.beta.kubernetes.io/"

	// ProfileRuntimeDefault is he profile specifying the runtime default.
	ProfileRuntimeDefault = "runtime/default"
	// ProfileNamePrefix is the prefix for specifying profiles loaded on the node.
	ProfileNamePrefix = "localhost/"
)
