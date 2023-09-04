package configs

import "golang.org/x/sys/unix"

type Mount struct {
	// Source path for the mount.
	Source string `json:"source"`

	// Destination path for the mount inside the container.
	Destination string `json:"destination"`

	// Device the mount is for.
	Device string `json:"device"`

	// Mount flags.
	Flags int `json:"flags"`

	// Propagation Flags
	PropagationFlags []int `json:"propagation_flags"`

	// Mount data applied to the mount.
	Data string `json:"data"`

	// Relabel source if set, "z" indicates shared, "Z" indicates unshared.
	Relabel string `json:"relabel"`

	// RecAttr represents mount properties to be applied recursively (AT_RECURSIVE), see mount_setattr(2).
	RecAttr *unix.MountAttr `json:"rec_attr"`

	// Extensions are additional flags that are specific to runc.
	Extensions int `json:"extensions"`

	// UIDMappings is used to changing file user owners w/o calling chown.
	// Note that, the underlying filesystem should support this feature to be
	// used.
	// Every mount point could have its own mapping.
	UIDMappings []IDMap `json:"uid_mappings,omitempty"`

	// GIDMappings is used to changing file group owners w/o calling chown.
	// Note that, the underlying filesystem should support this feature to be
	// used.
	// Every mount point could have its own mapping.
	GIDMappings []IDMap `json:"gid_mappings,omitempty"`
}

func (m *Mount) IsBind() bool {
	return m.Flags&unix.MS_BIND != 0
}

func (m *Mount) IsIDMapped() bool {
	return len(m.UIDMappings) > 0 || len(m.GIDMappings) > 0
}
