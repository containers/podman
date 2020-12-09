% podman-network-connect(1)

## NAME
podman\-network\-connect - Connect a container to a network

## SYNOPSIS
**podman network connect** [*options*] network container

## DESCRIPTION
Connects a container to a network. A container can be connected to a network by name or by ID.
Once connected, the container can communicate with other containers in the same network.

This command is not available for rootless users.

## OPTIONS
#### **--alias**
Add network-scoped alias for the container.  If the network is using the `dnsname` CNI plugin, these aliases
can be used for name resolution on the given network.  Multiple *--alias* options may be specified as input.

## EXAMPLE

Connect a container named *web* to a network named *test*
```
podman network connect test web
```

Connect a container name *web* to a network named *test* with two aliases: web1 and web2
```
podman network connect --alias web1 --alias web2 test web
```

## SEE ALSO
podman(1), podman-network(1), podman-network-disconnect(1), podman-network-inspect(1)

## HISTORY
November 2020, Originally compiled by Brent Baude <bbaude@redhat.com>
