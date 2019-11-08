% podman-rm(1)

## NAME
podman\-rm - Remove one or more containers

## SYNOPSIS
**podman rm** [*options*] *container*

**podman container rm** [*options*] *container*

## DESCRIPTION
**podman rm** will remove one or more containers from the host.  The container name or ID can be used.  This does not remove images.
Running or unusable containers will not be removed without the `-f` option.

## OPTIONS

**--all**, **-a**

Remove all containers.  Can be used in conjunction with -f as well.

**--force**, **-f**

Force the removal of running and paused containers. Forcing a container removal also
removes containers from container storage even if the container is not known to podman.
Containers could have been created by a different container engine.
In addition, forcing can be used to remove unusable containers, e.g. containers
whose OCI runtime has become unavailable.

**--latest**, **-l**

Instead of providing the container name or ID, use the last created container. If you use methods other than Podman
to run containers such as CRI-O, the last started container could be from either of those methods.

The latest option is not supported on the remote client.

**--storage**

Remove the container from the storage library only.
This is only possible with containers that are not present in libpod (cannot be seen by `podman ps`).
It is used to remove containers from `podman build` and `buildah`, and orphan containers which were only partially removed by `podman rm`.
The storage option conflicts with the **--all**, **--latest**, and **--volumes** options.

**--volumes**, **-v**

Remove anonymous volumes associated with the container. This does not include named volumes
created with `podman volume create`, or the `--volume` option of `podman run` and `podman create`.

## EXAMPLE
Remove a container by its name *mywebserver*
```
podman rm mywebserver
```
Remove several containers by name and container id.
```
podman rm mywebserver myflaskserver 860a4b23
```

Forcibly remove a container by container ID.
```
podman rm -f 860a4b23
```

Remove all containers regardless of its run state.
```
podman rm -f -a
```

Forcibly remove the latest container created.
```
podman rm -f --latest
```

## Exit Status
**_0_** if all specified containers removed
**_1_** if one of the specified containers did not exist, and no other failures
**_2_** if one of the specified containers is paused or running
**_125_** if the command fails for a reason other than container did not exist or is paused/running

## SEE ALSO
podman(1), podman-image-rm(1)

## HISTORY
August 2017, Originally compiled by Ryan Cole <rycole@redhat.com>
