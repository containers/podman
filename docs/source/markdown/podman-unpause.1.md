% podman-unpause(1)

## NAME
podman\-unpause - Unpause one or more containers

## SYNOPSIS
**podman unpause** [*options*]|[*container* ...]

**podman container unpause** [*options*]|[*container* ...]

## DESCRIPTION
Unpauses the processes in one or more containers.  You may use container IDs or names as input.

## OPTIONS

#### **--all**, **-a**

Unpause all paused containers.

## EXAMPLE

Unpause a container called 'mywebserver'
```
podman unpause mywebserver
```

Unpause a container by a partial container ID.
```
podman unpause 860a4b23
```

Unpause all **paused** containers.
```
podman unpause -a
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-pause(1)](podman-pause.1.md)**

## HISTORY
September 2017, Originally compiled by Dan Walsh <dwalsh@redhat.com>
