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

## EXAMPLES

Prune all containers, pods, and networks that are not in use.
```
$ podman system prune --all
WARNING! This command removes:
	- all stopped containers
	- all networks not used by at least one container
	- all images without at least one container associated with them
	- all build cache

Are you sure you want to continue? [y/N] y
Deleted Images
ce21f047f73644dcb9cd55ad247433fb47ade48ad4f4e676881fbcb2d5735c76
c874afc1f445044eaa821da0c32328707406e546a688d8c4c22587a5f88a1992
2fce09cfad57c6de112654eeb6f6da1851f3ced1cff7ac0002378642c2c7ca84
0cd6d2e072175191823999cd189f8d262ba5e460271095570d8cffb1d9072e9a
172bdaffe628cc7b7f8b7b6695438afc612a9833382f5968a06740a3804c3b64
bf07fec943ec23054f3b81c0e65926a1c83dc82c50933dc6372c60e09fdb2d4f
b2f735cbb571dd6a28e66455af0623ecc81f9c5b74259d3e04b3bac3b178e965
cea2ff433c610f5363017404ce989632e12b953114fefc6f597a58e813c15d61
Deleted Networks
podman-default-kube-network
Total reclaimed space: 3.372GB
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-system(1)](podman-system.1.md)**

## HISTORY
February 2019, Originally compiled by Dan Walsh (dwalsh at redhat dot com)
December 2020, converted filter information from docs.docker.com documentation by Dan Walsh (dwalsh at redhat dot com)
