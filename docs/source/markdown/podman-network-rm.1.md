% podman-network-rm(1)

## NAME
podman\-network\-rm - Remove one or more networks

## SYNOPSIS
**podman network rm** [*options*] [*network...*]

## DESCRIPTION
Delete one or more Podman networks.

## OPTIONS
#### **--force**, **-f**

The `force` option will remove all containers that use the named network. If the container is
running, the container will be stopped and removed.

#### **--time**, **-t**=*seconds*

Seconds to wait before forcibly stopping the running containers that are using the specified network. The --force option must be specified to use the --time option.

## EXAMPLE

Delete the `podman9` network

```
# podman network rm podman9
Deleted: podman9
```

Delete the `fred` network and all containers associated with the network.

```
# podman network rm -f fred
Deleted: fred
```

## Exit Status
  **0**   All specified networks removed

  **1**   One of the specified networks did not exist, and no other failures

  **2**   The network is in use by a container or a Pod

  **125** The command fails for any other reason

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-network(1)](podman-network.1.md)**

## HISTORY
August 2019, Originally compiled by Brent Baude <bbaude@redhat.com>
