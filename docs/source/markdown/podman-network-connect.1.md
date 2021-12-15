% podman-network-connect(1)

## NAME
podman\-network\-connect - Connect a container to a network

## SYNOPSIS
**podman network connect** [*options*] network container

## DESCRIPTION
Connects a container to a network. A container can be connected to a network by name or by ID.
Once connected, the container can communicate with other containers in the same network.

## OPTIONS
#### **--alias**=*name*
Add network-scoped alias for the container.  If the network is using the `dnsname` CNI plugin, these aliases
can be used for name resolution on the given network.  Multiple *--alias* options may be specified as input.
NOTE: A container will only have access to aliases on the first network that it joins.  This is a limitation
that will be removed in a later release.

#### **--ip**=*address*
Set a static ipv4 address for this container on this network.

#### **--ip6**=*address*
Set a static ipv6 address for this container on this network.

#### **--mac-address**=*address*
Set a static mac address for this container on this network.

## EXAMPLE

Connect a container named *web* to a network named *test*
```
podman network connect test web
```

Connect a container name *web* to a network named *test* with two aliases: web1 and web2
```
podman network connect --alias web1 --alias web2 test web
```

Connect a container name *web* to a network named *test* with a static ip.
```
podman network connect --ip 10.89.1.13 test web
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-network(1)](podman-network.1.md)**, **[podman-network-disconnect(1)](podman-network-disconnect.1.md)**

## HISTORY
November 2020, Originally compiled by Brent Baude <bbaude@redhat.com>
