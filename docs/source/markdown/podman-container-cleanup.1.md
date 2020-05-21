% podman-container-cleanup(1)

## NAME
podman\-container\-cleanup - Cleanup the container's network and mountpoints

## SYNOPSIS
**podman container cleanup** [*options*] *container*

## DESCRIPTION
**podman container cleanup** cleans up exited containers by removing all mountpoints and network configuration from the host.  The container name or ID can be used.  The cleanup command does not remove the containers.  Running containers will not be cleaned up.
Sometimes container's mount points and network stacks can remain if the podman command was killed or the container ran in daemon mode.  This command is automatically executed when you run containers in daemon mode by the conmon process when the container exits.

## OPTIONS

**--all**, **-a**

Cleanup all containers.

**--exec**=_session_

Clean up an exec session for a single container.
Can only be specified if a single container is being cleaned up (conflicts with **--all** as such).
If **--rm** is not specified, temporary files for the exec session will be cleaned up; if it is, the exec session will be removed from the container.
Conflicts with **--rmi** as the container is not being cleaned up so the image cannot be removed.

**--latest**, **-l**
Instead of providing the container name or ID, use the last created container. If you use methods other than Podman
to run containers such as CRI-O, the last started container could be from either of those methods.

The latest option is not supported on the remote client.

**--rm**

After cleanup, remove the container entirely.

**--rmi**

After cleanup, remove the image entirely.

## EXAMPLE

`podman container cleanup mywebserver`

`podman container cleanup mywebserver myflaskserver 860a4b23`

`podman container cleanup 860a4b23`

`podman container cleanup -a`

`podman container cleanup --latest`

## SEE ALSO
podman(1), podman-container(1)

## HISTORY
Jun 2018, Originally compiled by Dan Walsh <dwalsh@redhat.com>
