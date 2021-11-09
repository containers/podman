% podman-pause(1)

## NAME
podman\-pause - Pause one or more containers

## SYNOPSIS
**podman pause** [*options*] [*container*...]

**podman container pause** [*options*] [*container*...]

## DESCRIPTION
Pauses all the processes in one or more containers.  You may use container IDs or names as input.

## OPTIONS

#### **--all**, **-a**

Pause all running containers.

## EXAMPLE

Pause a container named 'mywebserver'
```
podman pause mywebserver
```

Pause a container by partial container ID.
```
podman pause 860a4b23
```

Pause all **running** containers.
```
podman pause -a
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-unpause(1)](podman-unpause.1.md)**

## HISTORY
September 2017, Originally compiled by Dan Walsh <dwalsh@redhat.com>
