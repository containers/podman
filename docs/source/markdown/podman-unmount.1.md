% podman-unmount 1

## NAME
podman\-unmount - Unmount a working container's root filesystem

## SYNOPSIS
**podman unmount** [*options*] *container* [...]

**podman umount** [*options*] *container* [...]

**podman container unmount** [*options*] *container* [...]

**podman container umount** [*options*] *container* [...]

## DESCRIPTION
Unmounts the specified containers' root file system, if no other processes
are using it.

Container storage increments a mount counter each time a container is mounted.
When a container is unmounted, the mount counter is decremented, and the
container's root filesystem is physically unmounted only when the mount
counter reaches zero indicating no other processes are using the mount.
An unmount can be forced with the --force flag.

Note: Podman can be used to unmount Podman containers as well as external containers.
External containers are containers created in container/storage by other tools like
Buildah and CRI-O.

## OPTIONS
#### **--all**, **-a**

All of the currently mounted containers will be unmounted.

#### **--force**, **-f**

Force the unmounting of specified containers' root file system, even if other
processes have mounted it.

Note: This could cause other processes that are using the file system to fail,
as the mount point could be removed without their knowledge.

#### **--latest**, **-l**

Instead of providing the container name or ID, use the last created container.
If you use methods other than Podman to run containers such as CRI-O, the last
started container could be from either of those methods. (This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)

## EXAMPLE

Unmount container with a given ID
```
podman container unmount containerID
```

Unmount multiple containers with given IDs
```
podman unmount containerID1 containerID2 containerID3
```

Unmount all containers
```
podman unmount --all
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-container-mount(1)](podman-container-mount.1.md)**, **[podman-image-mount(1)](podman-image-mount.1.md)**
