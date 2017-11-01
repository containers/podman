% kpod(1) kpod-mount - Mount a working container's root filesystem.
% Dan Walsh
# kpod-mount "1" "July 2017" "kpod"

## NAME
kpod mount - Mount a working container's root filesystem

## SYNOPSIS
**kpod** **mount**

**kpod** **mount** **containerID**

## DESCRIPTION
Mounts the specified container's root file system in a location which can be
accessed from the host, and returns its location.

If you execute the command without any arguments, the tool will list all of the
currently mounted containers.

## RETURN VALUE
The location of the mounted file system.  On error an empty string and errno is
returned.

## OPTIONS

**--format**
    Print the mounted containers in specified format (json)

**--notruncate**

Do not truncate IDs in output.

**--label**

SELinux label for the mount point

## EXAMPLE

kpod mount c831414b10a3

/var/lib/containers/storage/overlay/f3ac502d97b5681989dff84dfedc8354239bcecbdc2692f9a639f4e080a02364/merged

kpod mount

c831414b10a3 /var/lib/containers/storage/overlay/f3ac502d97b5681989dff84dfedc8354239bcecbdc2692f9a639f4e080a02364/merged

a7060253093b /var/lib/containers/storage/overlay/0ff7d7ca68bed1ace424f9df154d2dd7b5a125c19d887f17653cbcd5b6e30ba1/merged

## SEE ALSO
kpod(1), kpod-umount(1), mount(8)
