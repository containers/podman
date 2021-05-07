% podman-rmi(1)

## NAME
podman\-rmi - Removes one or more locally stored images

## SYNOPSIS
**podman rmi** [*options*] *image* [...]

**podman image rm** [*options*] *image* [...]

## DESCRIPTION
Removes one or more locally stored images.

## OPTIONS

#### **--all**, **-a**

Remove all images in the local storage.

#### **--force**, **-f**

This option will cause podman to remove all containers that are using the image before removing the image from the system.


Remove an image by its short ID
```
$ podman rmi c0ed59d05ff7
```
Remove an image and its associated containers.
```
$ podman rmi --force imageID
```

Remove multiple images by their shortened IDs.
```
$ podman rmi c4dfb1609ee2 93fd78260bd1 c0ed59d05ff7
```

Remove all images and containers.
```
$ podman rmi -a -f
```
## Exit Status
  **0**   All specified images removed

  **1**   One of the specified images did not exist, and no other failures

  **2**   One of the specified images has child images or is being used by a container

  **125** The command fails for any other reason

## SEE ALSO
podman(1)

## HISTORY
March 2017, Originally compiled by Dan Walsh <dwalsh@redhat.com>
