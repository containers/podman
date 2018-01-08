% podman(1) podman-exec - Execute a command in a running container
% Brent Baude
# podman-exec "1" "December 2017" "podman"

## NAME
podman exec - Execute a command in a running container

## SYNOPSIS
**podman exec**
**CONTAINER**
[COMMAND] [ARG...]
[**--help**|**-h**]

## DESCRIPTION
**podman exec** executes a command in a running container.

## OPTIONS
**--env, e**
You may specify arbitrary environment variables that are available for the
command to be executed.

**--interactive, -i**
Not supported.  All exec commands are interactive by default.

**--latest, -l**
Instead of providing the container name or ID, use the last created container. If you use methods other than Podman
to run containers such as CRI-O, the last started  container could be from either of those methods.

**--privileged**
Give the process extended Linux capabilities when running the command in container.

**--tty, -t**
Allocate a pseudo-TTY.

**--user, -u**
Sets the username or UID used and optionally the groupname or GID for the specified command.
The following examples are all valid:
--user [user | user:group | uid | uid:gid | user:gid | uid:group ]

## EXAMPLES


## SEE ALSO
podman(1), podman-run(1)

## HISTORY
December 2017, Originally compiled by Brent Baude<bbaude@redhat.com>
