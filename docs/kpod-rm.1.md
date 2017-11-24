% kpod(1) kpod-rm - Remove one or more containers
% Ryan Cole
# kpod-rm "1" "August 2017" "kpod"

## NAME
kpod rm - Remove one or more containers

## SYNOPSIS
**kpod** **rm** [*options* [...]] container

## DESCRIPTION
kpod rm will remove one or more containers from the host.  The container name or ID can be used.  This does not remove images.  Running containers will not be removed without the -f option

## OPTIONS

**--force, f**

Force the removal of a running container

**--all, a**

Remove all containers.  Can be used in conjunction with -f as well.

## EXAMPLE

kpod rm mywebserver

kpod rm mywebserver myflaskserver 860a4b23

kpod rm -f 860a4b23

kpod rm -f -a

## SEE ALSO
kpod(1), kpod-rmi(1)

## HISTORY
August 2017, Originally compiled by Ryan Cole <rycole@redhat.com>
