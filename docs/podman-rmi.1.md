% podman(1) podman-rmi - Removes one or more images
% Dan Walsh
# podman-rmi "1" "March 2017" "podman"

## NAME
podman rmi - Removes one or more images

## SYNOPSIS
**podman** **rmi** **imageID [...]**

## DESCRIPTION
Removes one or more locally stored images.

## OPTIONS

**-all**, **-a**

Remove all of the locally storage images
**--force, -f**

Executing this command will stop all containers that are using the image and remove them from the system

## EXAMPLE

podman rmi imageID

podman rmi --force imageID

podman rmi imageID1 imageID2 imageID3

podman rmi -a -f

## SEE ALSO
podman(1)

## HISTORY
March 2017, Originally compiled by Dan Walsh <dwalsh@redhat.com>
