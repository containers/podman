% podman(1) podman-start - Stop one or more containers
% Brent Baude
# podman-start "1" "November 2017" "podman"

## NAME
podman start - Start one or more containers

## SYNOPSIS
**podman start [OPTIONS] CONTAINER [...]**

## DESCRIPTION
Start one or more containers.  You may use container IDs or names as input.  The *attach* and *interactive*
options cannot be used to override the *--tty** and *--interactive* options from when the container
was created.

## OPTIONS

**--attach, -a**

Attach container's STDOUT and STDERR.  The default is false. This option cannot be used when
starting multiple containers.

**--detach-keys**

Override the key sequence for detaching a container. Format is a single character [a-Z] or
ctrl-<value> where <value> is one of: a-z, @, ^, [, , or _.

**--interactive, -i**

Attach container's STDIN. The default is false.


## EXAMPLE

podman start mywebserver

podman start 860a4b23 5421ab4

podman start -i -a 860a4b23

## SEE ALSO
podman(1), podman-create(1)

## HISTORY
November 2018, Originally compiled by Brent Baude <bbaude@redhat.com>
