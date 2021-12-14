% podman-image-unmount(1)

## NAME
podman\-image\-unmount - Unmount an image's root filesystem

## SYNOPSIS
**podman image unmount** [*options*] *image* [...]

**podman image umount** [*options*] *image* [...]

## DESCRIPTION
Unmounts the specified images' root file system, if no other processes
are using it.

Image storage increments a mount counter each time an image is mounted.
When a image is unmounted, the mount counter is decremented, and the
image's root filesystem is physically unmounted only when the mount
counter reaches zero indicating no other processes are using the mount.
An unmount can be forced with the --force flag.

## OPTIONS
#### **--all**, **-a**

All of the currently mounted images will be unmounted.

#### **--force**, **-f**

Force the unmounting of specified images' root file system, even if other
processes have mounted it.

Note: This could cause other processes that are using the file system to fail,
as the mount point could be removed without their knowledge.

## EXAMPLE

podman image unmount imageID

podman image unmount imageID1 imageID2 imageID3

podman image unmount --all

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-image-mount(1)](podman-image-mount.1.md)**, **[podman-container-mount(1)](podman-container-mount.1.md)**
