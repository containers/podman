# podman-manifest-rm "1" "April 2021" "podman"

## NAME
podman\-manifest\-rm - Remove manifest list or image index from local storage

## SYNOPSIS
**podman manifest rm** *list-or-index* [...]

## DESCRIPTION
Removes one or more locally stored manifest lists.

## EXAMPLE

podman manifest rm `<list>`

podman manifest rm listid1 listid2

**storage.conf** (`/etc/containers/storage.conf`)

storage.conf is the storage configuration file for all tools using containers/storage

The storage configuration file specifies all of the available container storage options for tools using shared container storage.

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-manifest(1)](podman-manifest.1.md)**,  **[containers-storage.conf(5)](https://github.com/containers/storage/blob/main/docs/containers-storage.conf.5.md)**
