% podman-system-prune 1

## NAME
podman\-system\-prune - Remove all unused pods, containers, images, networks, and volume data

## SYNOPSIS
**podman system prune** [*options*]

## DESCRIPTION
**podman system prune** removes all unused containers (both dangling and unreferenced), build containers, pods, networks, and optionally, volumes from local storage.

Use the **--all** option to delete all unused images.  Unused images are dangling images as well as any image that does not have any containers based on it.

By default, volumes are not removed to prevent important data from being deleted if there is currently no container using the volume. Use the **--volumes** flag when running the command to prune volumes as well.

By default, build containers are not removed to prevent interference with builds in progress. Use the **--build** flag when running the command to remove build containers as well.

## OPTIONS
#### **--all**, **-a**

Recursively remove all unused pods, containers, images, networks, and volume data. (Maximum 50 iterations.)

#### **--build**

Removes any build containers that were created during the build, but were not removed because the build was unexpectedly terminated.

Note: **This is not safe operation and should be executed only when no builds are in progress. It can interfere with builds in progress.**

#### **--external**

Tries to clean up remainders of previous containers or layers that are not references in the storage json files. These can happen in the case of unclean shutdowns or regular restarts in transient storage mode.

However, when using transient storage mode, the Podman database does not persist. This means containers leave the writable layers on disk after a reboot. When using a transient store, it is recommended that the **podman system prune --external** command is run during boot.

This option is incompatible with **--all** and **--filter** and drops the default behaviour of removing unused resources.

#### **--filter**=*filters*

Provide filter values.

The *filters* argument format is of `key=value`. If there is more than one *filter*, then pass multiple OPTIONS: **--filter** *foo=bar* **--filter** *bif=baz*.

Supported filters:

| Filter | Description                                                                                                     |
|:------:|-----------------------------------------------------------------------------------------------------------------|
| label  | Only remove containers and images, with (or without, in the case of label!=[...] is used) the specified labels. |
| until  | Only remove containers and images created before given timestamp.                                               |

The `label` *filter* accepts two formats. One is the `label`=*key* or `label`=*key*=*value*, which removes containers and images with the specified labels. The other format is the `label!`=*key* or `label!`=*key*=*value*, which removes containers and images without the specified labels.

The `until` *filter* can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. 10m, 1h30m) computed relative to the machine’s time.

#### **--force**, **-f**

Do not prompt for confirmation

#### **--help**, **-h**

Print usage statement

#### **--volumes**

Prune volumes currently unused by any container

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-system(1)](podman-system.1.md)**

## HISTORY
February 2019, Originally compiled by Dan Walsh (dwalsh at redhat dot com)
December 2020, converted filter information from docs.docker.com documentation by Dan Walsh (dwalsh at redhat dot com)
