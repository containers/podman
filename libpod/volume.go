package libpod

import (
	"time"

	"github.com/containers/libpod/libpod/lock"
)

// Volume is the type used to create named volumes
// TODO: all volumes should be created using this and the Volume API
type Volume struct {
	config *VolumeConfig

	valid   bool
	runtime *Runtime
	lock    lock.Locker
}

// VolumeConfig holds the volume's config information
type VolumeConfig struct {
	// Name of the volume.
	Name string `json:"name"`
	// ID of the volume's lock.
	LockID uint32 `json:"lockID"`
	// Labels for the volume.
	Labels map[string]string `json:"labels"`
	// The volume driver. Empty string or local does not activate a volume
	// driver, all other volumes will.
	Driver string `json:"volumeDriver"`
	// The location the volume is mounted at.
	MountPoint string `json:"mountPoint"`
	// Time the volume was created.
	CreatedTime time.Time `json:"createdAt,omitempty"`
	// Options to pass to the volume driver. For the local driver, this is
	// a list of mount options. For other drivers, they are passed to the
	// volume driver handling the volume.
	Options map[string]string `json:"volumeOptions"`
	// Whether this volume was created for a specific container and will be
	// removed with it.
	IsCtrSpecific bool `json:"ctrSpecific"`
	// UID the volume will be created as.
	UID int `json:"uid"`
	// GID the volume will be created as.
	GID int `json:"gid"`
}

// Name retrieves the volume's name
func (v *Volume) Name() string {
	return v.config.Name
}

// Driver retrieves the volume's driver.
func (v *Volume) Driver() string {
	return v.config.Driver
}

// Scope retrieves the volume's scope.
// Libpod does not implement volume scoping, and this is provided solely for
// Docker compatibility. It returns only "local".
func (v *Volume) Scope() string {
	return "local"
}

// Labels returns the volume's labels
func (v *Volume) Labels() map[string]string {
	labels := make(map[string]string)
	for key, value := range v.config.Labels {
		labels[key] = value
	}
	return labels
}

// MountPoint returns the volume's mountpoint on the host
func (v *Volume) MountPoint() string {
	return v.config.MountPoint
}

// Options return the volume's options
func (v *Volume) Options() map[string]string {
	options := make(map[string]string)
	for key, value := range v.config.Options {
		options[key] = value
	}

	return options
}

// IsCtrSpecific returns whether this volume was created specifically for a
// given container. Images with this set to true will be removed when the
// container is removed with the Volumes parameter set to true.
func (v *Volume) IsCtrSpecific() bool {
	return v.config.IsCtrSpecific
}

// UID returns the UID the volume will be created as.
func (v *Volume) UID() int {
	return v.config.UID
}

// GID returns the GID the volume will be created as.
func (v *Volume) GID() int {
	return v.config.GID
}

// CreatedTime returns the time the volume was created at. It was not tracked
// for some time, so older volumes may not contain one.
func (v *Volume) CreatedTime() time.Time {
	return v.config.CreatedTime
}
