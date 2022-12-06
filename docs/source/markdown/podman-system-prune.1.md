% podman-system-prune 1

## NAME
podman\-system\-prune - Remove all unused pods, containers, images, networks, and volume data

## SYNOPSIS
**podman system prune** [*options*]

## DESCRIPTION
**podman system prune** removes all unused containers (both dangling and unreferenced), pods, networks, and optionally, volumes from local storage.

Use the **--all** option to delete all unused images.  Unused images are dangling images as well as any image that does not have any containers based on it.

By default, volumes are not removed to prevent important data from being deleted if there is currently no container using the volume. Use the **--volumes** flag when running the command to prune volumes as well.

## OPTIONS
#### **--all**, **-a**

Recursively remove all unused pods, containers, images, networks, and volume data. (Maximum 50 iterations.)

#### **--external**

Removes all leftover container storage files from local storage that are not managed by podman. In normal circumstances no such data should exist, but in case of an unclean shutdown the podman database may be corrupted and cause his.

However, when using transient storage mode, the Podman database does not persist. This means containers will leave the writable layers on disk after a reboot. When using a transient store, it is recommended that the **podman system prune --external** command is run during boot.

This option is incompatible with **--all** and **--filter** and drops the default behaviour of removing unused resources.

#### **--filter**=*filters*

Provide filter values.

The *filters* argument format is of `key=value`. If there is more than one *filter*, then pass multiple OPTIONS: **--filter** *foo=bar* **--filter** *bif=baz*.

Supported filters:

| Filter             | Description                                                                 |
| :----------------: | --------------------------------------------------------------------------- |
| *label*            | Only remove containers and images, with (or without, in the case of label!=[...] is used) the specified labels.                  |
| *until*            | Only remove containers and images created before given timestamp.           |

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
