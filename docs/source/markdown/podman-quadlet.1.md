% podman-quadlet 1

## NAME
podman\-quadlet - Allows users to manage Quadlets

## SYNOPSIS
**podman quadlet** *subcommand*

## DESCRIPTION
`podman quadlet` is a set of subcommands that manage Quadlets.

Podman Quadlets allow users to manage containers, pods, volumes, networks, and images declaratively via systemd unit files, streamlining container management on Linux systems without the complexity of full orchestration tools like Kubernetes


## SUBCOMMANDS

| Command | Man Page                                                   | Description                                                  |
|---------|------------------------------------------------------------|--------------------------------------------------------------|
| install | [podman-quadlet-install(1)](podman-quadlet-install.1.md)   | Install a quadlet file or quadlet application                |
| list    | [podman-quadlet-list(1)](podman-quadlet-list.1.md)         | List installed quadlets                                      |
| print   | [podman-quadlet-print(1)](podman-quadlet-print.1.md)       | Display the contents of a quadlet                            |
| rm      | [podman-quadlet-rm(1)](podman-quadlet-rm.1.md)             | Removes an installed quadlet                                 |

## SEE ALSO
**[podman(1)](podman.1.md)**
