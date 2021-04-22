package libimage

import "time"

// EventType indicates the type of an event.  Currrently, there is only one
// supported type for container image but we may add more (e.g., for manifest
// lists) in the future.
type EventType int

const (
	// EventTypeUnknow is an unitialized EventType.
	EventTypeUnknown EventType = iota
	// EventTypeImagePull represents an image pull.
	EventTypeImagePull
	// EventTypeImagePush represents an image push.
	EventTypeImagePush
	// EventTypeImageRemove represents an image removal.
	EventTypeImageRemove
	// EventTypeImageLoad represents an image being loaded.
	EventTypeImageLoad
	// EventTypeImageSave represents an image being saved.
	EventTypeImageSave
	// EventTypeImageTag represents an image being tagged.
	EventTypeImageTag
	// EventTypeImageUntag represents an image being untagged.
	EventTypeImageUntag
	// EventTypeImageMount represents an image being mounted.
	EventTypeImageMount
	// EventTypeImageUnmounted represents an image being unmounted.
	EventTypeImageUnmount
)

// Event represents an event such an image pull or image tag.
type Event struct {
	// ID of the object (e.g., image ID).
	ID string
	// Name of the object (e.g., image name "quay.io/containers/podman:latest")
	Name string
	// Time of the event.
	Time time.Time
	// Type of the event.
	Type EventType
}
