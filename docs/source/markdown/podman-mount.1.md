% podman-mount(1)

## NAME
podman\-mount - Mount a working container's root filesystem

## SYNOPSIS
**podman mount** [*options*] [*container* ...]

**podman container mount** [*options*] [*container* ...]

## DESCRIPTION
Mounts the specified containers' root file system in a location which can be
accessed from the host, and returns its location.

If you execute the command without any arguments, Podman will list all of the
currently mounted containers, including external containers. External containers are
containers in container/storage by tools other than Podman. For example Buildah and
CRI-O.

Rootless mode only supports mounting VFS driver, unless you enter the user namespace
via the `podman unshare` command. All other storage drivers will fail to mount.

## RETURN VALUE
The location of the mounted file system.  On error an empty string and errno is
returned.

## OPTIONS

#### **--all**, **-a**

Mount all podman containers. (External containers will not be mounted)

#### **--format**=*format*

Print the mounted containers in specified format (json).

#### **--latest**, **-l**

Instead of providing the container name or ID, use the last created container.
If you use methods other than Podman to run containers such as CRI-O, the last
started container could be from either of those methods. (This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)

#### **--no-trunc**

Do not truncate the output (default *false*).

## EXAMPLE

```
podman mount c831414b10a3

/var/lib/containers/storage/overlay/f3ac502d97b5681989dff84dfedc8354239bcecbdc2692f9a639f4e080a02364/merged
```

```
podman mount

c831414b10a3 /var/lib/containers/storage/overlay/f3ac502d97b5681989dff84dfedc8354239bcecbdc2692f9a639f4e080a02364/merged
a7060253093b /var/lib/containers/storage/overlay/0ff7d7ca68bed1ace424f9df154d2dd7b5a125c19d887f17653cbcd5b6e30ba1/merged
```
```
podman mount c831414b10a3 a7060253093b

/var/lib/containers/storage/overlay/f3ac502d97b5681989dff84dfedc8354239bcecbdc2692f9a639f4e080a02364/merged
/var/lib/containers/storage/overlay/0ff7d7ca68bed1ace424f9df154d2dd7b5a125c19d887f17653cbcd5b6e30ba1/merged
```

```
podman mount

c831414b10a3 /var/lib/containers/storage/overlay/f3ac502d97b5681989dff84dfedc8354239bcecbdc2692f9a639f4e080a02364/merged
a7060253093b /var/lib/containers/storage/overlay/0ff7d7ca68bed1ace424f9df154d2dd7b5a125c19d887f17653cbcd5b6e30ba1/merged
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-umount(1)](podman-umount.1.md)**, **[podman-unshare(1)](podman-unshare.1.md)**, **mount(8)**
