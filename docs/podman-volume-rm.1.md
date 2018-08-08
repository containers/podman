% podman-volume-rm(1)

## NAME
podman\-volume\-rm - Remove one or more volumes

## SYNOPSIS
**podman volume rm** [*options*]

## DESCRIPTION

Removes one ore more volumes. Only volumes that are not being used will be removed.
If a volume is being used by a container, an error will be returned unless the **--force**
flag is being used. To remove all the volumes, use the **--all** flag.


## OPTIONS

**-a**, **--all**=""

Remove all volumes.

**-f**, **--force**=""

Remove a volume by force, even if it is being used by a container

**--help**

Print usage statement


## EXAMPLES

```
$ podman volume rm myvol1 myvol2

$ podman volume rm --all

$ podman volume rm --force myvol
```

## SEE ALSO
podman-volume(1)

## HISTORY
November 2018, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
