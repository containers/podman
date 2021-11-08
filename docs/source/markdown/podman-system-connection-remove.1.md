% podman-system-connection-remove(1)

## NAME
podman\-system\-connection\-remove - Delete named destination

## SYNOPSIS
**podman system connection remove** [*options*] *name*

## DESCRIPTION
Delete named ssh destination.

## OPTIONS

#### **--all**=*false*, **-a**

Remove all connections.

## EXAMPLE
```
$ podman system connection remove production
```
## SEE ALSO
podman-system(1) , podman-system-connection(1) , containers.conf(5)

## HISTORY
July 2020, Originally compiled by Jhon Honce (jhonce at redhat dot com)
