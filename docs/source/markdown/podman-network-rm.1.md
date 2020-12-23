% podman-network-rm(1)

## NAME
podman\-network\-rm - Remove one or more CNI networks

## SYNOPSIS
**podman network rm** [*options*] [*network...*]

## DESCRIPTION
Delete one or more Podman networks.

## OPTIONS
#### **--force**, **-f**

The `force` option will remove all containers that use the named network. If the container is
running, the container will be stopped and removed.

## EXAMPLE

Delete the `cni-podman9` network

```
# podman network rm cni-podman9
Deleted: cni-podman9
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
podman(1), podman-network(1), podman-network-inspect(1)

## HISTORY
August 2019, Originally compiled by Brent Baude <bbaude@redhat.com>
