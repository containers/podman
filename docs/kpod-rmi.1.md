% kpod(1) kpod-rmi - Removes one or more images
% Dan Walsh
# kpod-rmi "1" "March 2017" "kpod"

## NAME
kpod rmi - Removes one or more images

## SYNOPSIS
**kpod** **rmi** **imageID [...]**

## DESCRIPTION
Removes one or more locally stored images.

## OPTIONS

**--force, -f**

Executing this command will stop all containers that are using the image and remove them from the system

## EXAMPLE

kpod rmi imageID

kpod rmi --force imageID

kpod rmi imageID1 imageID2 imageID3

## SEE ALSO
kpod(1)

## HISTORY
March 2017, Originally compiled by Dan Walsh <dwalsh@redhat.com>
