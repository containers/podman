% podman-rmi "1"

## NAME
podman\-rmi - Removes one or more images

## SYNOPSIS
**podman rmi** *image* ...

## DESCRIPTION
Removes one or more locally stored images.

## OPTIONS

**-all**, **-a**

Remove all images in the local storage.

**--force**, **-f**

This option will cause podman to remove all containers that are using the image before removing the image from the system.

## EXAMPLE

podman rmi imageID

podman rmi --force imageID

podman rmi imageID1 imageID2 imageID3

podman rmi -a -f

## SEE ALSO
podman(1)

## HISTORY
March 2017, Originally compiled by Dan Walsh <dwalsh@redhat.com>
