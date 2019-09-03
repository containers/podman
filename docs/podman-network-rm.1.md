% podman-network-rm(1)

## NAME
podman\-network\-rm - Remove one or more CNI networks

## SYNOPSIS
**podman network rm**  [*network...*]

## DESCRIPTION
Delete one or more Podman networks.

## EXAMPLE

Delete the `podman9` network

```
# podman network rm podman
Deleted: podman9
```

## SEE ALSO
podman(1), podman-network(1), podman-network-inspect(1)

## HISTORY
August 2019, Originally compiled by Brent Baude <bbaude@redhat.com>
