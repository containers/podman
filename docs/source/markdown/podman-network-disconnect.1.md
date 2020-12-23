% podman-network-disconnect(1)

## NAME
podman\-network\-disconnect - Disconnect a container from a network

## SYNOPSIS
**podman network disconnect** [*options*] network container

## DESCRIPTION
Disconnects a container from a network.

This command is not available for rootless users.

## OPTIONS
#### **--force**, **-f**

Force the container to disconnect from a network

## EXAMPLE
Disconnect a container named *web* from a network called *test*.

```
podman network disconnect test web
```


## SEE ALSO
podman(1), podman-network(1), podman-network-connect(1)

## HISTORY
November 2020, Originally compiled by Brent Baude <bbaude@redhat.com>
