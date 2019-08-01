package config

// ThinpoolOptionsConfig represents the "storage.options.thinpool"
// TOML config table.
type ThinpoolOptionsConfig struct {
	// AutoExtendPercent determines the amount by which pool needs to be
	// grown. This is specified in terms of % of pool size. So a value of
	// 20 means that when threshold is hit, pool will be grown by 20% of
	// existing pool size.
	AutoExtendPercent string `toml:"autoextend_percent"`

	// AutoExtendThreshold determines the pool extension threshold in terms
	// of percentage of pool size. For example, if threshold is 60, that
	// means when pool is 60% full, threshold has been hit.
	AutoExtendThreshold string `toml:"autoextend_threshold"`

	// BaseSize specifies the size to use when creating the base device,
	// which limits the size of images and containers.
	BaseSize string `toml:"basesize"`

	// BlockSize specifies a custom blocksize to use for the thin pool.
	BlockSize string `toml:"blocksize"`

	// DirectLvmDevice specifies a custom block storage device to use for
	// the thin pool.
	DirectLvmDevice string `toml:"directlvm_device"`

	// DirectLvmDeviceForcewipes device even if device already has a
	// filesystem
	DirectLvmDeviceForce string `toml:"directlvm_device_force"`

	// Fs specifies the filesystem type to use for the base device.
	Fs string `toml:"fs"`

	// log_level sets the log level of devicemapper.
	LogLevel string `toml:"log_level"`

	// MinFreeSpace specifies the min free space percent in a thin pool
	// require for new device creation to
	MinFreeSpace string `toml:"min_free_space"`

	// MkfsArg specifies extra mkfs arguments to be used when creating the
	// basedevice.
	MkfsArg string `toml:"mkfsarg"`

	// MountOpt specifies extra mount options used when mounting the thin
	// devices.
	MountOpt string `toml:"mountopt"`

	// UseDeferredDeletion marks device for deferred deletion
	UseDeferredDeletion string `toml:"use_deferred_deletion"`

	// UseDeferredRemoval marks device for deferred removal
	UseDeferredRemoval string `toml:"use_deferred_removal"`

	// XfsNoSpaceMaxRetriesFreeSpace specifies the maximum number of
	// retries XFS should attempt to complete IO when ENOSPC (no space)
	// error is returned by underlying storage device.
	XfsNoSpaceMaxRetries string `toml:"xfs_nospace_max_retries"`
}

// OptionsConfig represents the "storage.options" TOML config table.
type OptionsConfig struct {
	// AdditionalImagesStores is the location of additional read/only
	// Image stores.  Usually used to access Networked File System
	// for shared image content
	AdditionalImageStores []string `toml:"additionalimagestores"`

	// Size
	Size string `toml:"size"`

	// RemapUIDs is a list of default UID mappings to use for layers.
	RemapUIDs string `toml:"remap-uids"`
	// RemapGIDs is a list of default GID mappings to use for layers.
	RemapGIDs string `toml:"remap-gids"`
	// IgnoreChownErrors is a flag for whether chown errors should be
	// ignored when building an image.
	IgnoreChownErrors string `toml:"ignore_chown_errors"`

	// RemapUser is the name of one or more entries in /etc/subuid which
	// should be used to set up default UID mappings.
	RemapUser string `toml:"remap-user"`
	// RemapGroup is the name of one or more entries in /etc/subgid which
	// should be used to set up default GID mappings.
	RemapGroup string `toml:"remap-group"`
	// Thinpool container options to be handed to thinpool drivers
	Thinpool struct{ ThinpoolOptionsConfig } `toml:"thinpool"`
	// OSTree repository
	OstreeRepo string `toml:"ostree_repo"`

	// Do not create a bind mount on the storage home
	SkipMountHome string `toml:"skip_mount_home"`

	// Alternative program to use for the mount of the file system
	MountProgram string `toml:"mount_program"`

	// MountOpt specifies extra mount options used when mounting
	MountOpt string `toml:"mountopt"`
}
