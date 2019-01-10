package libpod

import "github.com/containers/storage"

// Volume is the type used to create named volumes
// TODO: all volumes should be created using this and the Volume API
type Volume struct {
	config *VolumeConfig

	valid   bool
	runtime *Runtime
	lock    storage.Locker
}

// VolumeConfig holds the volume's config information
//easyjson:json
type VolumeConfig struct {
	Name       string            `json:"name"`
	Labels     map[string]string `json:"labels"`
	MountPoint string            `json:"mountPoint"`
	Driver     string            `json:"driver"`
	Options    map[string]string `json:"options"`
	Scope      string            `json:"scope"`
}

// Name retrieves the volume's name
func (v *Volume) Name() string {
	return v.config.Name
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

// Driver returns the volume's driver
func (v *Volume) Driver() string {
	return v.config.Driver
}

// Options return the volume's options
func (v *Volume) Options() map[string]string {
	options := make(map[string]string)
	for key, value := range v.config.Options {
		options[key] = value
	}

	return options
}

// Scope returns the scope of the volume
func (v *Volume) Scope() string {
	return v.config.Scope
}
