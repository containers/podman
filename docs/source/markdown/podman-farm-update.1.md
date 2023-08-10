% podman-farm-update 1

## NAME
podman\-farm\-update - Update an existing farm

## SYNOPSIS
**podman farm update** [*options*] *name*

## DESCRIPTION
Update a farm by either adding connections to it, removing connections from it, or setting it as the new
default farm.

## OPTIONS

#### **--add**, **-a**

Add new connections to an existing farm. Multiple connections can be added at once.

#### **--default**, **-d**

Set the current farm as the default.

#### **--remove**, **-r**

Remove one or more connections from an existing farm.

## EXAMPLE
```
$ podman farm update --add f35,f38 farm1

$ podman farm update --remove f35 farm1

$ podman farm update --default farm2
```
## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-farm(1)](podman-farm.1.md)**

## HISTORY
July 2023, Originally compiled by Urvashi Mohnani (umohnani at redhat dot com)s
