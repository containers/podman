% podman-volume-mount 1

## NAME
podman\-volume\-mount - Mount a volume filesystem

## SYNOPSIS
**podman volume mount** [*volume* ...]

## DESCRIPTION
Mounts the specified volumes' file system in a location which can be
accessed from the host, and returns its location.

Rootless mode only supports mounting file volumes unless Podman is run within the user namespace
via the `podman unshare` command. All other volume types will fail to mount.

## RETURN VALUE
The location of the mounted file system.  On error an empty string and errno is
returned.

## EXAMPLE

```
podman volume mount foo
/home/dwalsh/.local/share/containers/storage/volumes/foo/_data
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-volume(1)](podman-volume.1.md)**, **[podman-volume-unmount(1)](podman-volume-unmount.1.md)**, **[podman-unshare(1)](podman-unshare.1.md)**, **mount(8)**
