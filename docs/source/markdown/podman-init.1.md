% podman-init 1

## NAME
podman\-init - Initialize one or more containers

## SYNOPSIS
**podman init** [*options*] *container* [*container*...]

**podman container init** [*options*] *container* [*container*...]

## DESCRIPTION
Initialize one or more containers.
You may use container IDs or names as input.
Initializing a container performs all tasks necessary for starting the container (mounting filesystems, creating an OCI spec, initializing the container network) but does not start the container.
If a container is not initialized, the `podman start` and `podman run` commands will do so automatically prior to starting it.
This command is intended to be used for inspecting or modifying the container's filesystem or OCI spec prior to starting it.
This can be used to inspect the container before it runs, or debug why a container is failing to run.

## OPTIONS

#### **--all**, **-a**

Initialize all containers. Containers that have already initialized (including containers that have been started and are running) are ignored.

#### **--latest**, **-l**

Instead of providing the container name or ID, use the last created container. If you use methods other than Podman
to run containers such as CRI-O, the last started container could be from either of those methods. (This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)

## EXAMPLE

Initialize container with a given ID
```
podman init 35480fc9d568
```

Initialize container with a given name
```
podman init test1
```

Initialize the latest container created by Podman
```
podman init --latest
```
## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-start(1)](podman-start.1.md)**

## HISTORY
April 2019, Originally compiled by Matthew Heon <mheon@redhat.com>
