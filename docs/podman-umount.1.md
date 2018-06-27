% podman-umount "1"

## NAME
podman\-umount - Unmount the specified working containers' root file system.

## SYNOPSIS
**podman** **umount** **containerID [...]**

## DESCRIPTION
Unmounts the specified containers' root file system.

## OPTIONS
**--all, -a**

All of the currently mounted containers will be unmounted.

## EXAMPLE

podman umount containerID

podman umount containerID1 containerID2 containerID3

podman umount --all

## SEE ALSO
podman(1), podman-mount(1)
