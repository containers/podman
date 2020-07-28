package libpod

import (
	"time"

	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/libpod/lock"
)

// Volume is a libpod named volume.
// Named volumes may be shared by multiple containers, and may be created using
// more complex options than normal bind mounts. They may be backed by a mounted
// filesystem on the host.
type Volume struct {
	config *VolumeConfig
	state  *VolumeState

	valid   bool
	runtime *Runtime
	lock    lock.Locker
}

// VolumeConfig holds the volume's immutable configuration.
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
	Options map[string]string `json:"volumeOptions,omitempty"`
	// Whether this volume is anonymous (will be removed on container exit)
	IsAnon bool `json:"isAnon"`
	// UID the volume will be created as.
	UID int `json:"uid"`
	// GID the volume will be created as.
	GID int `json:"gid"`
}

// VolumeState holds the volume's mutable state.
// Volumes are not guaranteed to have a state. Only volumes using the Local
// driver that have mount options set will create a state.
type VolumeState struct {
	// MountCount is the number of times this volume has been requested to
	// be mounted.
	// It is incremented on mount() and decremented on unmount().
	// On incrementing from 0, the volume will be mounted on the host.
	// On decrementing to 0, the volume will be unmounted on the host.
	MountCount uint `json:"mountCount"`
	// NeedsCopyUp indicates that the next time the volume is mounted into
	// a container, the container will "copy up" the contents of the
	// mountpoint into the volume.
	// This should only be done once. As such, this is set at container
	// create time, then cleared after the copy up is done and never set
	// again.
	NeedsCopyUp bool `json:"notYetMounted,omitempty"`
	// NeedsChown indicates that the next time the volume is mounted into
	// a container, the container will chown the volume to the container process
	// UID/GID.
	NeedsChown bool `json:"notYetChowned,omitempty"`
	// UIDChowned is the UID the volume was chowned to.
	UIDChowned int `json:"uidChowned,omitempty"`
	// GIDChowned is the GID the volume was chowned to.
	GIDChowned int `json:"gidChowned,omitempty"`
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
	for k, v := range v.config.Options {
		options[k] = v
	}
	return options
}

// Anonymous returns whether this volume is anonymous. Anonymous volumes were
// created with a container, and will be removed when that container is removed.
func (v *Volume) Anonymous() bool {
	return v.config.IsAnon
}

// UID returns the UID the volume will be created as.
func (v *Volume) UID() (int, error) {
	v.lock.Lock()
	defer v.lock.Unlock()

	if !v.valid {
		return -1, define.ErrVolumeRemoved
	}

	if v.state.UIDChowned > 0 {
		return v.state.UIDChowned, nil
	}
	return v.config.UID, nil
}

// GID returns the GID the volume will be created as.
func (v *Volume) GID() (int, error) {
	v.lock.Lock()
	defer v.lock.Unlock()

	if !v.valid {
		return -1, define.ErrVolumeRemoved
	}

	if v.state.GIDChowned > 0 {
		return v.state.GIDChowned, nil
	}
	return v.config.GID, nil
}

// CreatedTime returns the time the volume was created at. It was not tracked
// for some time, so older volumes may not contain one.
func (v *Volume) CreatedTime() time.Time {
	return v.config.CreatedTime
}

// Config returns the volume's configuration.
func (v *Volume) Config() (*VolumeConfig, error) {
	config := VolumeConfig{}
	err := JSONDeepCopy(v.config, &config)
	return &config, err
}

// VolumeInUse goes through the container dependencies of a volume
// and checks if the volume is being used by any container.
func (v *Volume) VolumeInUse() ([]string, error) {
	v.lock.Lock()
	defer v.lock.Unlock()

	if !v.valid {
		return nil, define.ErrVolumeRemoved
	}
	return v.runtime.state.VolumeInUse(v)
}

// IsDangling returns whether this volume is dangling (unused by any
// containers).
func (v *Volume) IsDangling() (bool, error) {
	ctrs, err := v.VolumeInUse()
	if err != nil {
		return false, err
	}
	return len(ctrs) == 0, nil
}
