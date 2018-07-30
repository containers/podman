% podman-umount "1"

## NAME
podman\-umount - Unmount the specified working containers' root file system.

## SYNOPSIS
**podman umount** *container* ...

## DESCRIPTION
Unmounts the specified containers' root file system, if no other processes
are using it.

Container storage increments a mount counter each time a container is mounted.
When a container is unmounted, the mount counter is decremented and the
container's root filesystem is physically unmounted only when the mount
counter reaches zero indicating no other processes are using the mount.
An unmount can be forced with the --force flag.

## OPTIONS
**--all, -a**

All of the currently mounted containers will be unmounted.

**--force, -f**

Force the unmounting of specified containers' root file system, even if other
processes have mounted it.

Note: This could cause other processes that are using the file system to fail,
as the mount point could be removed without their knowledge.

## EXAMPLE

podman umount containerID

podman umount containerID1 containerID2 containerID3

podman umount --all

## SEE ALSO
podman(1), podman-mount(1)
