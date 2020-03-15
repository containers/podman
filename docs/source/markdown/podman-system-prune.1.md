% podman-system-prune(1)

## NAME
podman\-system\-prune - Remove all unused container, image and volume data

## SYNOPSIS
**podman system prune** [*options*]

## DESCRIPTION
**podman system prune** removes all unused containers (both dangling and unreferenced), pods and optionally, volumes from local storage.

With the **--all** option, you can delete all unused images.  Unused images are dangling images as well as any image that does not have any containers based on it.

By default, volumes are not removed to prevent important data from being deleted if there is currently no container using the volume. Use the **--volumes** flag when running the command to prune volumes as well.

## OPTIONS
**--all**, **-a**

Remove all unused images not just dangling ones.

**--force**, **-f**

Do not prompt for confirmation

**--help**, **-h**

Print usage statement

**--volumes**

Prune volumes currently unused by any container

## SEE ALSO
podman(1), podman-image-prune(1), podman-container-prune(1), podman-pod-prune(1), podman-volume-prune(1)

## HISTORY
February 2019, Originally compiled by Dan Walsh (dwalsh at redhat dot com)
