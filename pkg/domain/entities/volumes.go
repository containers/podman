package entities

import "time"

// swagger:model VolumeCreate
type VolumeCreateOptions struct {
	// New volume's name. Can be left blank
	Name string `schema:"name"`
	// Volume driver to use
	Driver string `schema:"driver"`
	// User-defined key/value metadata.
	Label map[string]string `schema:"label"`
	// Mapping of driver options and values.
	Options map[string]string `schema:"opts"`
}

type IdOrNameResponse struct {
	// The Id or Name of an object
	IdOrName string
}

type VolumeConfigResponse struct {
	// Name of the volume.
	Name   string            `json:"name"`
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
	Options map[string]string `json:"volumeOptions,omitempty"`
	// UID the volume will be created as.
	UID int `json:"uid"`
	// GID the volume will be created as.
	GID int `json:"gid"`
}
