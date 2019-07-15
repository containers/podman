package libpod

import (
	"time"
)

// Volume is the type used to create named volumes
// TODO: all volumes should be created using this and the Volume API
type Volume struct {
	config *VolumeConfig

	valid   bool
	runtime *Runtime
}

// VolumeConfig holds the volume's config information
type VolumeConfig struct {
	// Name of the volume
	Name string `json:"name"`

	Labels        map[string]string `json:"labels"`
	Driver        string            `json:"driver"`
	MountPoint    string            `json:"mountPoint"`
	CreatedTime   time.Time         `json:"createdAt,omitempty"`
	Options       map[string]string `json:"options"`
	IsCtrSpecific bool              `json:"ctrSpecific"`
	UID           int               `json:"uid"`
	GID           int               `json:"gid"`
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
// Docker compatability. It returns only "local".
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
