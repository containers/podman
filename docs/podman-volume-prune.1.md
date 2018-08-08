% podman-volume-prune(1)

## NAME
podman\-volume\-prune - Remove all unused volumes

## SYNOPSIS
**podman volume rm** [*options*]

## DESCRIPTION

Removes all unused volumes. You will be prompted to confirm the removal of all the
unused volumes. To bypass the confirmation, use the **--force** flag.


## OPTIONS

**-f**, **--force**=""

Do not prompt for confirmation.

**--help**

Print usage statement


## EXAMPLES

```
$ podman volume prune

$ podman volume prune --force
```

## SEE ALSO
podman-volume(1)

## HISTORY
November 2018, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
