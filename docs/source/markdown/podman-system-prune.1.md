% podman-system-prune(1)

## NAME
podman\-system\-prune - Remove all unused pod, container, image and volume data

## SYNOPSIS
**podman system prune** [*options*]

## DESCRIPTION
**podman system prune** removes all unused containers (both dangling and unreferenced), pods and optionally, volumes from local storage.

With the **--all** option, you can delete all unused images.  Unused images are dangling images as well as any image that does not have any containers based on it.

By default, volumes are not removed to prevent important data from being deleted if there is currently no container using the volume. Use the **--volumes** flag when running the command to prune volumes as well.

## OPTIONS
#### **--all**, **-a**

Recursively remove all unused pod, container, image and volume data (Maximum 50 iterations.)

#### **--filter**=*filters*

Provide filter values.

The --filter flag format is of “key=value”. If there is more than one filter, then pass multiple flags (e.g., --filter "foo=bar" --filter "bif=baz")

Supported filters:

- `until` (_timestamp_) - only remove containers and images created before given timestamp
- `label` (label=_key_, label=_key=value_, label!=_key_, or label!=_key=value_) - only remove containers and images, with (or without, in case label!=... is used) the specified labels.

The until filter can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. 10m, 1h30m) computed relative to the machine’s time.

The label filter accepts two formats. One is the label=... (label=_key_ or label=_key=value_), which removes containers and images with the specified labels. The other format is the label!=... (label!=_key_ or label!=_key=value_), which removes containers and images without the specified labels.

#### **--force**, **-f**

Do not prompt for confirmation

#### **--help**, **-h**

Print usage statement

#### **--volumes**

Prune volumes currently unused by any container

## SEE ALSO
podman(1), podman-image-prune(1), podman-container-prune(1), podman-pod-prune(1), podman-volume-prune(1)

## HISTORY
February 2019, Originally compiled by Dan Walsh (dwalsh at redhat dot com)
December 2020, converted filter information from docs.docker.com documentation by Dan Walsh (dwalsh at redhat dot com)
