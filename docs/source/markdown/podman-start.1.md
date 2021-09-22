% podman-start(1)

## NAME
podman\-start - Start one or more containers

## SYNOPSIS
**podman start** [*options*] *container* ...

**podman container start** [*options*] *container* ...

## DESCRIPTION
Start one or more containers.  You may use container IDs or names as input.  The *attach* and *interactive*
options cannot be used to override the *--tty* and *--interactive* options from when the container
was created. If you attempt to start a running container with the *--attach* option, podman will simply
attach to the container.

## OPTIONS

#### **--attach**, **-a**

Attach container's STDOUT and STDERR.  The default is false. This option cannot be used when
starting multiple containers.

#### **--detach-keys**=*sequence*

Specify the key sequence for detaching a container. Format is a single character `[a-Z]` or one or more `ctrl-<value>` characters where `<value>` is one of: `a-z`, `@`, `^`, `[`, `,` or `_`. Specifying "" will disable this feature. The default is *ctrl-p,ctrl-q*.

#### **--interactive**, **-i**

Attach container's STDIN. The default is false.

#### **--latest**, **-l**

Instead of providing the container name or ID, use the last created container. If you use methods other than Podman
to run containers such as CRI-O, the last started container could be from either of those methods. (This option is not available with the remote Podman client)

#### **--sig-proxy**

Proxy received signals to the process (non-TTY mode only). SIGCHLD, SIGSTOP, and SIGKILL are not proxied. The default is *true* when attaching, *false* otherwise.

#### **--all**

Start all the containers created by Podman, default is only running containers.

#### **--filter**, **-f**

Filter what containers are going to be started from the given arguments.
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


## EXAMPLE

podman start mywebserver

podman start 860a4b231279 5421ab43b45

podman start --interactive --attach 860a4b231279

podman start -i -l

## SEE ALSO
podman(1), podman-create(1)

## HISTORY
November 2018, Originally compiled by Brent Baude <bbaude@redhat.com>
