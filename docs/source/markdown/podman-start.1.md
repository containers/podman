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

**--attach**, **-a**

Attach container's STDOUT and STDERR.  The default is false. This option cannot be used when
starting multiple containers.

**--detach-keys**=*sequence*

Specify the key sequence for detaching a container. Format is a single character `[a-Z]` or one or more `ctrl-<value>` characters where `<value>` is one of: `a-z`, `@`, `^`, `[`, `,` or `_`. Specifying "" will disable this feature. The default is *ctrl-p,ctrl-q*.

**--interactive**, **-i**

Attach container's STDIN. The default is false.

**--latest**, **-l**

Instead of providing the container name or ID, use the last created container. If you use methods other than Podman
to run containers such as CRI-O, the last started container could be from either of those methods.

The latest option is not supported on the remote client.

**--sig-proxy**=*true|false*

Proxy received signals to the process (non-TTY mode only). SIGCHLD, SIGSTOP, and SIGKILL are not proxied. The default is *true* when attaching, *false* otherwise.

## EXAMPLE

podman start mywebserver

podman start 860a4b231279 5421ab43b45

podman start --interactive --attach 860a4b231279

podman start -i -l

## SEE ALSO
podman(1), podman-create(1)

## HISTORY
November 2018, Originally compiled by Brent Baude <bbaude@redhat.com>
