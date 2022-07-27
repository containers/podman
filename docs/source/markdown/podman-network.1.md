% podman-network(1)

## NAME
podman\-network - Manage Podman networks

## SYNOPSIS
**podman network** *subcommand*

## DESCRIPTION
The network command manages networks for Podman.

Podman supports two network backends [Netavark](https://github.com/containers/netavark)
and [CNI](https://www.cni.dev/). Support for netavark was added in Podman v4.0. To configure
the network backend use the `network_backend`key under the `[Network]` in
**[containers.conf(5)](https://github.com/containers/common/blob/master/docs/containers.conf.5.md)**.
New systems should use netavark by default, to check what backed is used run
`podman info --format {{.Host.NetworkBackend}}`.

All network commands work for both backends but CNI and Netavark use different config files
so networks have to be created again after a backend change.

## COMMANDS

| Command    | Man Page                                                       | Description                                                     |
| ---------- | -------------------------------------------------------------- | --------------------------------------------------------------- |
| connect    | [podman-network-connect(1)](podman-network-connect.1.md)       | Connect a container to a network                                |
| create     | [podman-network-create(1)](podman-network-create.1.md)         | Create a Podman network                                         |
| disconnect | [podman-network-disconnect(1)](podman-network-disconnect.1.md) | Disconnect a container from a network                           |
| exists     | [podman-network-exists(1)](podman-network-exists.1.md)         | Check if the given network exists                               |
| inspect    | [podman-network-inspect(1)](podman-network-inspect.1.md)       | Displays the network configuration for one or more networks     |
| ls         | [podman-network-ls(1)](podman-network-ls.1.md)                 | Display a summary of networks                                   |
| prune      | [podman-network-prune(1)](podman-network-prune.1.md)           | Remove all unused networks                                      |
| reload     | [podman-network-reload(1)](podman-network-reload.1.md)         | Reload network configuration for containers                     |
| rm         | [podman-network-rm(1)](podman-network-rm.1.md)                 | Remove one or more networks                                     |

## SEE ALSO
**[podman(1)](podman.1.md)**, **[containers.conf(5)](https://github.com/containers/common/blob/main/docs/containers.conf.5.md)**
