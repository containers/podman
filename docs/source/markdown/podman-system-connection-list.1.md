% podman-system-connection-list(1)

## NAME
podman\-system\-connection\-list - List the destination for the Podman service(s)

## SYNOPSIS
**podman system connection list**

**podman system connection ls**

## DESCRIPTION
List ssh destination(s) for podman service(s).

## EXAMPLE
```
$ podman system connection list
Name URI                                           Identity
devl ssh://root@example.com/run/podman/podman.sock ~/.ssh/id_rsa
```
## SEE ALSO
podman-system(1) , containers.conf(5)

## HISTORY
July 2020, Originally compiled by Jhon Honce (jhonce at redhat dot com)
