% podman(1) podman-rm - Remove one or more containers
% Ryan Cole
# podman-rm "1" "August 2017" "podman"

## NAME
podman rm - Remove one or more containers

## SYNOPSIS
**podman** **rm** [*options* [...]] container

## DESCRIPTION
podman rm will remove one or more containers from the host.  The container name or ID can be used.  This does not remove images.  Running containers will not be removed without the -f option

## OPTIONS

**--force, f**

Force the removal of a running container

**--all, a**

Remove all containers.  Can be used in conjunction with -f as well.

**--latest, -l**
Instead of providing the container name or ID, use the last created container. If you use methods other than Podman
to run containers such as CRI-O, the last started container could be from either of those methods.
## EXAMPLE

podman rm mywebserver

podman rm mywebserver myflaskserver 860a4b23

podman rm -f 860a4b23

podman rm -f -a

podman rm -f --latest

## SEE ALSO
podman(1), podman-rmi(1)

## HISTORY
August 2017, Originally compiled by Ryan Cole <rycole@redhat.com>
