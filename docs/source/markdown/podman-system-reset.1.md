% podman-system-reset(1)

## NAME
podman\-system\-reset - Reset storage back to initial state

## SYNOPSIS
**podman system reset** [*options*]

## DESCRIPTION
**podman system reset** removes all pods, containers, images and volumes. Must be run after changing any of the following values in the `containers.conf` file: `static_dir`, `tmp_dir` or `volume_path`.

## OPTIONS
**--force**, **-f**

Do not prompt for confirmation

**--help**, **-h**

Print usage statement

## SEE ALSO
`podman(1)`, `podman-system(1)`

## HISTORY
November 2019, Originally compiled by Dan Walsh (dwalsh at redhat dot com)
