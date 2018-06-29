% podman-container-cleanup "1"

## NAME
podman\-container\-cleanup - Cleanup Container storage and networks

## SYNOPSIS
**podman container cleanup [OPTIONS] CONTAINER**

## DESCRIPTION
`podman container cleanup` cleans up exited containers by removing all mountpoints and network configuration from the host.  The container name or ID can be used.  The cleanup command does not remove the containers.  Running containers will not be cleaned up.
Sometimes container's mount points and network stacks can remain if the podman command was killed or the container ran in daemon mode.  This command is automatically executed when you run containers in daemon mode by the conmon process when the container exits.

## OPTIONS

**--all, a**

Cleanup all containers.

**--latest, -l**
Instead of providing the container name or ID, use the last created container. If you use methods other than Podman
to run containers such as CRI-O, the last started container could be from either of those methods.
## EXAMPLE

`podman container cleanup mywebserver`

`podman container cleanup mywebserver myflaskserver 860a4b23`

`podman container cleanup 860a4b23`

`podman container-cleanup -a`

`podman container cleanup --latest`

## SEE ALSO
podman(1), podman-container(1)

## HISTORY
Jun 2018, Originally compiled by Dan Walsh <dwalsh@redhat.com>
