package config

import (
	"fmt"
	"os"
)

// ThinpoolOptionsConfig represents the "storage.options.thinpool"
// TOML config table.
type ThinpoolOptionsConfig struct {
	// AutoExtendPercent determines the amount by which pool needs to be
	// grown. This is specified in terms of % of pool size. So a value of
	// 20 means that when threshold is hit, pool will be grown by 20% of
	// existing pool size.
	AutoExtendPercent string `toml:"autoextend_percent,omitempty"`

	// AutoExtendThreshold determines the pool extension threshold in terms
	// of percentage of pool size. For example, if threshold is 60, that
	// means when pool is 60% full, threshold has been hit.
	AutoExtendThreshold string `toml:"autoextend_threshold,omitempty"`

	// BaseSize specifies the size to use when creating the base device,
	// which limits the size of images and containers.
	BaseSize string `toml:"basesize,omitempty"`

	// BlockSize specifies a custom blocksize to use for the thin pool.
	BlockSize string `toml:"blocksize,omitempty"`

	// DirectLvmDevice specifies a custom block storage device to use for
	// the thin pool.
	DirectLvmDevice string `toml:"directlvm_device,omitempty"`

	// DirectLvmDeviceForcewipes device even if device already has a
	// filesystem
	DirectLvmDeviceForce string `toml:"directlvm_device_force,omitempty"`

	// Fs specifies the filesystem type to use for the base device.
	Fs string `toml:"fs,omitempty"`

	// log_level sets the log level of devicemapper.
	LogLevel string `toml:"log_level,omitempty"`

	// MetadataSize specifies the size of the metadata for the thinpool
	// It will be used with the `pvcreate --metadata` option.
	MetadataSize string `toml:"metadatasize,omitempty"`

	// MinFreeSpace specifies the min free space percent in a thin pool
	// require for new device creation to
	MinFreeSpace string `toml:"min_free_space,omitempty"`

	// MkfsArg specifies extra mkfs arguments to be used when creating the
	// basedevice.
	MkfsArg string `toml:"mkfsarg,omitempty"`

	// MountOpt specifies extra mount options used when mounting the thin
	// devices.
	MountOpt string `toml:"mountopt,omitempty"`

	// Size
	Size string `toml:"size,omitempty"`

	// UseDeferredDeletion marks device for deferred deletion
	UseDeferredDeletion string `toml:"use_deferred_deletion,omitempty"`

	// UseDeferredRemoval marks device for deferred removal
	UseDeferredRemoval string `toml:"use_deferred_removal,omitempty"`

	// XfsNoSpaceMaxRetriesFreeSpace specifies the maximum number of
	// retries XFS should attempt to complete IO when ENOSPC (no space)
	// error is returned by underlying storage device.
	XfsNoSpaceMaxRetries string `toml:"xfs_nospace_max_retries,omitempty"`
}

type AufsOptionsConfig struct {
	// MountOpt specifies extra mount options used when mounting
	MountOpt string `toml:"mountopt,omitempty"`
}

type BtrfsOptionsConfig struct {
	// MinSpace is the minimal spaces allocated to the device
	MinSpace string `toml:"min_space,omitempty"`
	// Size
	Size string `toml:"size,omitempty"`
}

type OverlayOptionsConfig struct {
	// IgnoreChownErrors is a flag for whether chown errors should be
	// ignored when building an image.
	IgnoreChownErrors string `toml:"ignore_chown_errors,omitempty"`
	// MountOpt specifies extra mount options used when mounting
	MountOpt string `toml:"mountopt,omitempty"`
	// Alternative program to use for the mount of the file system
	MountProgram string `toml:"mount_program,omitempty"`
	// Size
	Size string `toml:"size,omitempty"`
	// Inodes is used to set a maximum inodes of the container image.
	Inodes string `toml:"inodes,omitempty"`
	// Do not create a bind mount on the storage home
	SkipMountHome string `toml:"skip_mount_home,omitempty"`
	// ForceMask indicates the permissions mask (e.g. "0755") to use for new
	// files and directories
	ForceMask string `toml:"force_mask,omitempty"`
}

type VfsOptionsConfig struct {
	// IgnoreChownErrors is a flag for whether chown errors should be
	// ignored when building an image.
	IgnoreChownErrors string `toml:"ignore_chown_errors,omitempty"`
}

type ZfsOptionsConfig struct {
	// MountOpt specifies extra mount options used when mounting
	MountOpt string `toml:"mountopt,omitempty"`
	// Name is the File System name of the ZFS File system
	Name string `toml:"fsname,omitempty"`
	// Size
	Size string `toml:"size,omitempty"`
}

// OptionsConfig represents the "storage.options" TOML config table.
type OptionsConfig struct {
	// AdditionalImagesStores is the location of additional read/only
	// Image stores.  Usually used to access Networked File System
	// for shared image content
	AdditionalImageStores []string `toml:"additionalimagestores,omitempty"`

	// ImageStore is the location of image store which is separated from the
	// container store. Usually this is not recommended unless users wants
	// separate store for image and containers.
	ImageStore string `toml:"imagestore,omitempty"`

	// AdditionalLayerStores is the location of additional read/only
	// Layer stores.  Usually used to access Networked File System
	// for shared image content
	// This API is experimental and can be changed without bumping the
	// major version number.
	AdditionalLayerStores []string `toml:"additionallayerstores,omitempty"`

	// Size
	Size string `toml:"size,omitempty"`

	// RemapUIDs is a list of default UID mappings to use for layers.
	RemapUIDs string `toml:"remap-uids,omitempty"`
	// RemapGIDs is a list of default GID mappings to use for layers.
	RemapGIDs string `toml:"remap-gids,omitempty"`
	// IgnoreChownErrors is a flag for whether chown errors should be
	// ignored when building an image.
	IgnoreChownErrors string `toml:"ignore_chown_errors,omitempty"`

	// ForceMask indicates the permissions mask (e.g. "0755") to use for new
	// files and directories.
	ForceMask os.FileMode `toml:"force_mask,omitempty"`

	// RemapUser is the name of one or more entries in /etc/subuid which
	// should be used to set up default UID mappings.
	RemapUser string `toml:"remap-user,omitempty"`
	// RemapGroup is the name of one or more entries in /etc/subgid which
	// should be used to set up default GID mappings.
	RemapGroup string `toml:"remap-group,omitempty"`

	// RootAutoUsernsUser is the name of one or more entries in /etc/subuid and
	// /etc/subgid which should be used to set up automatically a userns.
	RootAutoUsernsUser string `toml:"root-auto-userns-user,omitempty"`

	// AutoUsernsMinSize is the minimum size for a user namespace that is
	// created automatically.
	AutoUsernsMinSize uint32 `toml:"auto-userns-min-size,omitempty"`

	// AutoUsernsMaxSize is the maximum size for a user namespace that is
	// created automatically.
	AutoUsernsMaxSize uint32 `toml:"auto-userns-max-size,omitempty"`

	// Aufs container options to be handed to aufs drivers
	Aufs struct{ AufsOptionsConfig } `toml:"aufs,omitempty"`

	// Btrfs container options to be handed to btrfs drivers
	Btrfs struct{ BtrfsOptionsConfig } `toml:"btrfs,omitempty"`

	// Thinpool container options to be handed to thinpool drivers
	Thinpool struct{ ThinpoolOptionsConfig } `toml:"thinpool,omitempty"`

	// Overlay container options to be handed to overlay drivers
	Overlay struct{ OverlayOptionsConfig } `toml:"overlay,omitempty"`

	// Vfs container options to be handed to VFS drivers
	Vfs struct{ VfsOptionsConfig } `toml:"vfs,omitempty"`

	// Zfs container options to be handed to ZFS drivers
	Zfs struct{ ZfsOptionsConfig } `toml:"zfs,omitempty"`

	// Do not create a bind mount on the storage home
	SkipMountHome string `toml:"skip_mount_home,omitempty"`

	// Alternative program to use for the mount of the file system
	MountProgram string `toml:"mount_program,omitempty"`

	// MountOpt specifies extra mount options used when mounting
	MountOpt string `toml:"mountopt,omitempty"`

	// PullOptions specifies options to be handed to pull managers
	// This API is experimental and can be changed without bumping the major version number.
	PullOptions map[string]string `toml:"pull_options,omitempty"`

	// DisableVolatile doesn't allow volatile mounts when it is set.
	DisableVolatile bool `toml:"disable-volatile,omitempty"`
}

// GetGraphDriverOptions returns the driver specific options
func GetGraphDriverOptions(driverName string, options OptionsConfig) []string {
	var doptions []string
	switch driverName {
	case "aufs":
		if options.Aufs.MountOpt != "" {
			return append(doptions, fmt.Sprintf("%s.mountopt=%s", driverName, options.Aufs.MountOpt))
		} else if options.MountOpt != "" {
			doptions = append(doptions, fmt.Sprintf("%s.mountopt=%s", driverName, options.MountOpt))
		}

	case "btrfs":
		if options.Btrfs.MinSpace != "" {
			return append(doptions, fmt.Sprintf("%s.min_space=%s", driverName, options.Btrfs.MinSpace))
		}
		if options.Btrfs.Size != "" {
			doptions = append(doptions, fmt.Sprintf("%s.size=%s", driverName, options.Btrfs.Size))
		} else if options.Size != "" {
			doptions = append(doptions, fmt.Sprintf("%s.size=%s", driverName, options.Size))
		}

	case "devicemapper":
		if options.Thinpool.AutoExtendPercent != "" {
			doptions = append(doptions, fmt.Sprintf("dm.thinp_autoextend_percent=%s", options.Thinpool.AutoExtendPercent))
		}
		if options.Thinpool.AutoExtendThreshold != "" {
			doptions = append(doptions, fmt.Sprintf("dm.thinp_autoextend_threshold=%s", options.Thinpool.AutoExtendThreshold))
		}
		if options.Thinpool.BaseSize != "" {
			doptions = append(doptions, fmt.Sprintf("dm.basesize=%s", options.Thinpool.BaseSize))
		}
		if options.Thinpool.BlockSize != "" {
			doptions = append(doptions, fmt.Sprintf("dm.blocksize=%s", options.Thinpool.BlockSize))
		}
		if options.Thinpool.DirectLvmDevice != "" {
			doptions = append(doptions, fmt.Sprintf("dm.directlvm_device=%s", options.Thinpool.DirectLvmDevice))
		}
		if options.Thinpool.DirectLvmDeviceForce != "" {
			doptions = append(doptions, fmt.Sprintf("dm.directlvm_device_force=%s", options.Thinpool.DirectLvmDeviceForce))
		}
		if options.Thinpool.Fs != "" {
			doptions = append(doptions, fmt.Sprintf("dm.fs=%s", options.Thinpool.Fs))
		}
		if options.Thinpool.LogLevel != "" {
			doptions = append(doptions, fmt.Sprintf("dm.libdm_log_level=%s", options.Thinpool.LogLevel))
		}
		if options.Thinpool.MetadataSize != "" {
			doptions = append(doptions, fmt.Sprintf("dm.metadata_size=%s", options.Thinpool.MetadataSize))
		}
		if options.Thinpool.MinFreeSpace != "" {
			doptions = append(doptions, fmt.Sprintf("dm.min_free_space=%s", options.Thinpool.MinFreeSpace))
		}
		if options.Thinpool.MkfsArg != "" {
			doptions = append(doptions, fmt.Sprintf("dm.mkfsarg=%s", options.Thinpool.MkfsArg))
		}
		if options.Thinpool.MountOpt != "" {
			doptions = append(doptions, fmt.Sprintf("%s.mountopt=%s", driverName, options.Thinpool.MountOpt))
		} else if options.MountOpt != "" {
			doptions = append(doptions, fmt.Sprintf("%s.mountopt=%s", driverName, options.MountOpt))
		}

		if options.Thinpool.Size != "" {
			doptions = append(doptions, fmt.Sprintf("%s.size=%s", driverName, options.Thinpool.Size))
		} else if options.Size != "" {
			doptions = append(doptions, fmt.Sprintf("%s.size=%s", driverName, options.Size))
		}

		if options.Thinpool.UseDeferredDeletion != "" {
			doptions = append(doptions, fmt.Sprintf("dm.use_deferred_deletion=%s", options.Thinpool.UseDeferredDeletion))
		}
		if options.Thinpool.UseDeferredRemoval != "" {
			doptions = append(doptions, fmt.Sprintf("dm.use_deferred_removal=%s", options.Thinpool.UseDeferredRemoval))
		}
		if options.Thinpool.XfsNoSpaceMaxRetries != "" {
			doptions = append(doptions, fmt.Sprintf("dm.xfs_nospace_max_retries=%s", options.Thinpool.XfsNoSpaceMaxRetries))
		}

	case "overlay", "overlay2":
		if options.Overlay.IgnoreChownErrors != "" {
			doptions = append(doptions, fmt.Sprintf("%s.ignore_chown_errors=%s", driverName, options.Overlay.IgnoreChownErrors))
		} else if options.IgnoreChownErrors != "" {
			doptions = append(doptions, fmt.Sprintf("%s.ignore_chown_errors=%s", driverName, options.IgnoreChownErrors))
		}
		if options.Overlay.MountProgram != "" {
			doptions = append(doptions, fmt.Sprintf("%s.mount_program=%s", driverName, options.Overlay.MountProgram))
		} else if options.MountProgram != "" {
			doptions = append(doptions, fmt.Sprintf("%s.mount_program=%s", driverName, options.MountProgram))
		}
		if options.Overlay.MountOpt != "" {
			doptions = append(doptions, fmt.Sprintf("%s.mountopt=%s", driverName, options.Overlay.MountOpt))
		} else if options.MountOpt != "" {
			doptions = append(doptions, fmt.Sprintf("%s.mountopt=%s", driverName, options.MountOpt))
		}
		if options.Overlay.Size != "" {
			doptions = append(doptions, fmt.Sprintf("%s.size=%s", driverName, options.Overlay.Size))
		} else if options.Size != "" {
			doptions = append(doptions, fmt.Sprintf("%s.size=%s", driverName, options.Size))
		}
		if options.Overlay.Inodes != "" {
			doptions = append(doptions, fmt.Sprintf("%s.inodes=%s", driverName, options.Overlay.Inodes))
		}
		if options.Overlay.SkipMountHome != "" {
			doptions = append(doptions, fmt.Sprintf("%s.skip_mount_home=%s", driverName, options.Overlay.SkipMountHome))
		} else if options.SkipMountHome != "" {
			doptions = append(doptions, fmt.Sprintf("%s.skip_mount_home=%s", driverName, options.SkipMountHome))
		}
		if options.Overlay.ForceMask != "" {
			doptions = append(doptions, fmt.Sprintf("%s.force_mask=%s", driverName, options.Overlay.ForceMask))
		} else if options.ForceMask != 0 {
			doptions = append(doptions, fmt.Sprintf("%s.force_mask=%s", driverName, options.ForceMask))
		}
	case "vfs":
		if options.Vfs.IgnoreChownErrors != "" {
			doptions = append(doptions, fmt.Sprintf("%s.ignore_chown_errors=%s", driverName, options.Vfs.IgnoreChownErrors))
		} else if options.IgnoreChownErrors != "" {
			doptions = append(doptions, fmt.Sprintf("%s.ignore_chown_errors=%s", driverName, options.IgnoreChownErrors))
		}

	case "zfs":
		if options.Zfs.Name != "" {
			doptions = append(doptions, fmt.Sprintf("%s.fsname=%s", driverName, options.Zfs.Name))
		}
		if options.Zfs.MountOpt != "" {
			doptions = append(doptions, fmt.Sprintf("%s.mountopt=%s", driverName, options.Zfs.MountOpt))
		} else if options.MountOpt != "" {
			doptions = append(doptions, fmt.Sprintf("%s.mountopt=%s", driverName, options.MountOpt))
		}
		if options.Zfs.Size != "" {
			doptions = append(doptions, fmt.Sprintf("%s.size=%s", driverName, options.Zfs.Size))
		} else if options.Size != "" {
			doptions = append(doptions, fmt.Sprintf("%s.size=%s", driverName, options.Size))
		}
	}
	return doptions
}
