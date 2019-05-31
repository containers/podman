% podman-exec(1)

## NAME
podman\-exec - Execute a command in a running container

## SYNOPSIS
**podman exec** [*options*] *container* [*command* [*arg* ...]]

## DESCRIPTION
**podman exec** executes a command in a running container.

## OPTIONS
**--env**, **-e**

You may specify arbitrary environment variables that are available for the
command to be executed.

**--interactive**, **-i**

Not supported.  All exec commands are interactive by default.

**--latest**, **-l**

Instead of providing the container name or ID, use the last created container. If you use methods other than Podman
to run containers such as CRI-O, the last started  container could be from either of those methods.

The latest option is not supported on the remote client.

**--preserve-fds**=*N*

Pass down to the process N additional file descriptors (in addition to 0, 1, 2).  The total FDs will be 3+N.

**--privileged**

Give the process extended Linux capabilities when running the command in container.

**--tty**, **-t**

Allocate a pseudo-TTY.

**--user**, **-u**

Sets the username or UID used and optionally the groupname or GID for the specified command.
The following examples are all valid:
--user [user | user:group | uid | uid:gid | user:gid | uid:group ]

**--workdir**, **-w**=*path*

Working directory inside the container

The default working directory for running binaries within a container is the root directory (/).
The image developer can set a different default with the WORKDIR instruction, which can be overridden
when creating the container.

## EXAMPLES

$ podman exec -it ctrID ls
$ podman exec -it -w /tmp myCtr pwd
$ podman exec --user root ctrID ls

## SEE ALSO
podman(1), podman-run(1)

## HISTORY
December 2017, Originally compiled by Brent Baude<bbaude@redhat.com>
