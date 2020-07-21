% podman-system-connection-rename(1)

## NAME
podman\-system\-connection\-rename - Rename the destination for Podman service

## SYNOPSIS
**podman system connection rename** *old* *new*

## DESCRIPTION
Rename ssh destination from *old* to *new*.

## EXAMPLE
```
$ podman system connection rename laptop devel
```
## SEE ALSO
podman-system(1) , podman-system-connection(1) , containers.conf(5)

## HISTORY
July 2020, Originally compiled by Jhon Honce (jhonce at redhat dot com)
