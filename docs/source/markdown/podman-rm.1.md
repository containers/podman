% podman-rm(1)

## NAME
podman\-rm - Remove one or more containers

## SYNOPSIS
**podman rm** [*options*] *container*

**podman container rm** [*options*] *container*

## DESCRIPTION
**podman rm** will remove one or more containers from the host.  The container name or ID can be used.  This does not remove images.
Running or unusable containers will not be removed without the **-f** option.

## OPTIONS

#### **--all**, **-a**

Remove all containers.  Can be used in conjunction with **-f** as well.

#### **--depend**

Remove selected container and recursively remove all containers that depend on it.

#### **--cidfile**

Read container ID from the specified file and remove the container.  Can be specified multiple times.

#### **--force**, **-f**

Force the removal of running and paused containers. Forcing a container removal also
removes containers from container storage even if the container is not known to podman.
Containers could have been created by a different container engine.
In addition, forcing can be used to remove unusable containers, e.g. containers
whose OCI runtime has become unavailable.

#### **--ignore**, **-i**

Ignore errors when specified containers are not in the container store.  A user
might have decided to manually remove a container which would lead to a failure
during the ExecStop directive of a systemd service referencing that container.

#### **--latest**, **-l**

Instead of providing the container name or ID, use the last created container. If you use methods other than Podman
to run containers such as CRI-O, the last started container could be from either of those methods. (This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)

#### **--time**, **-t**=*seconds*

Seconds to wait before forcibly stopping the container. The --force option must be specified to use the --time option.

#### **--volumes**, **-v**

Remove anonymous volumes associated with the container. This does not include named volumes
created with **podman volume create**, or the **--volume** option of **podman run** and **podman create**.

## EXAMPLE
Remove a container by its name *mywebserver*
```
$ podman rm mywebserver
```

Remove a *mywebserver* container and all of the containers that depend on it
```
$ podman rm --depend mywebserver
```

Remove several containers by name and container id.
```
$ podman rm mywebserver myflaskserver 860a4b23
```

Remove several containers reading their IDs from files.
```
$ podman rm --cidfile ./cidfile-1 --cidfile /home/user/cidfile-2
```

Forcibly remove a container by container ID.
```
$ podman rm -f 860a4b23
```

Remove all containers regardless of its run state.
```
$ podman rm -f -a
```

Forcibly remove the latest container created.
```
$ podman rm -f --latest
```

## Exit Status
  **0**   All specified containers removed

  **1**   One of the specified containers did not exist, and no other failures

  **2**   One of the specified containers is paused or running

  **125** The command fails for any other reason

## SEE ALSO
**[podman(1)](podman.1.md)**, **[crio(8)](https://github.com/cri-o/cri-o/blob/main/docs/crio.8.md)**

## HISTORY
August 2017, Originally compiled by Ryan Cole <rycole@redhat.com>
