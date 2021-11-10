% podman-network(1)

## NAME
podman\-network - Manage Podman CNI networks

## SYNOPSIS
**podman network** *subcommand*

## DESCRIPTION
The network command manages CNI networks for Podman.

## COMMANDS

| Command    | Man Page                                                       | Description                                                         |
| ---------- | -------------------------------------------------------------- | ------------------------------------------------------------------- |
| connect    | [podman-network-connect(1)](podman-network-connect.1.md)       | Connect a container to a network                                    |
| create     | [podman-network-create(1)](podman-network-create.1.md)         | Create a Podman CNI network                                         |
| disconnect | [podman-network-disconnect(1)](podman-network-disconnect.1.md) | Disconnect a container from a network                               |
| exists     | [podman-network-exists(1)](podman-network-exists.1.md)         | Check if the given network exists                                   |
| inspect    | [podman-network-inspect(1)](podman-network-inspect.1.md)       | Displays the raw CNI network configuration for one or more networks |
| ls         | [podman-network-ls(1)](podman-network-ls.1.md)                 | Display a summary of CNI networks                                   |
| prune      | [podman-network-prune(1)](podman-network-prune.1.md)           | Remove all unused networks |
| reload     | [podman-network-reload(1)](podman-network-reload.1.md)         | Reload network configuration for containers                         |
| rm         | [podman-network-rm(1)](podman-network-rm.1.md)                 | Remove one or more CNI networks                                     |

## SEE ALSO
**[podman(1)](podman.1.md)**
