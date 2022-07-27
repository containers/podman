% podman-stop(1)

## NAME
podman\-stop - Stop one or more running containers

## SYNOPSIS
**podman stop** [*options*] *container* ...

**podman container stop** [*options*] *container* ...

## DESCRIPTION
Stops one or more containers.  You may use container IDs or names as input. The **--time** switch
allows you to specify the number of seconds to wait before forcibly stopping the container after the stop command
is issued to the container. The default is 10 seconds. By default, containers are stopped with SIGTERM
and then SIGKILL after the timeout. The SIGTERM default can be overridden by the image used to create the
container and also via command line when creating the container.

## OPTIONS

#### **--all**, **-a**

Stop all running containers.  This does not include paused containers.

#### **--cidfile**

Read container ID from the specified file and remove the container.  Can be specified multiple times.

#### **--filter**, **-f**=*filter*

Filter what containers are going to be stopped.
Multiple filters can be given with multiple uses of the --filter flag.
Filters with the same key work inclusive with the only exception being
`label` which is exclusive. Filters with different keys always work exclusive.

Valid filters are listed below:

| **Filter**      | **Description**                                                                  |
| --------------- | -------------------------------------------------------------------------------- |
| id              | [ID] Container's ID (accepts regex)                                              |
| name            | [Name] Container's name (accepts regex)                                          |
| label           | [Key] or [Key=Value] Label assigned to a container                               |
| exited          | [Int] Container's exit code                                                      |
| status          | [Status] Container's status: 'created', 'exited', 'paused', 'running', 'unknown' |
| ancestor        | [ImageName] Image or descendant used to create container                         |
| before          | [ID] or [Name] Containers created before this container                          |
| since           | [ID] or [Name] Containers created since this container                           |
| volume          | [VolumeName] or [MountpointDestination] Volume mounted in container              |
| health          | [Status] healthy or unhealthy                                                    |
| pod             | [Pod] name or full or partial ID of pod                                          |
| network         | [Network] name or full ID of network                                             |

#### **--ignore**, **-i**

Ignore errors when specified containers are not in the container store.  A user
might have decided to manually remove a container which would lead to a failure
during the ExecStop directive of a systemd service referencing that container.

#### **--latest**, **-l**

Instead of providing the container name or ID, use the last created container. If you use methods other than Podman
to run containers such as CRI-O, the last started container could be from either of those methods. (This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)

#### **--time**, **-t**=*seconds*

Seconds to wait before forcibly stopping the container

## EXAMPLES

$ podman stop mywebserver

$ podman stop 860a4b235279

$ podman stop mywebserver 860a4b235279

$ podman stop --cidfile /home/user/cidfile-1

$ podman stop --cidfile /home/user/cidfile-1 --cidfile ./cidfile-2

$ podman stop --time 2 860a4b235279

$ podman stop -a

$ podman stop --latest

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-rm(1)](podman-rm.1.md)**

## HISTORY
September 2018, Originally compiled by Brent Baude <bbaude@redhat.com>
