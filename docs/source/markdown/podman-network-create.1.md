% podman-network-create(1)

## NAME
podman\-network-create - Create a Podman CNI network

## SYNOPSIS
**podman network create**  [*options*] name

## DESCRIPTION
Create a CNI-network configuration for use with Podman. By default, Podman creates a bridge connection. A
*Macvlan* connection can be created with the *macvlan* option. In the case of *Macvlan* connections, the
CNI *dhcp* plugin needs to be activated or the container image must have a DHCP client to interact
with the host network's DHCP server.

If no options are provided, Podman will assign a free subnet and name for your network.

Upon completion of creating the network, Podman will display the path to the newly added network file.

## OPTIONS
**--disable-dns**

Disables the DNS plugin for this network which if enabled, can perform container to container name
resolution.

**-d**, **--driver**

Driver to manage the network (default "bridge").  Currently only `bridge` is supported.

**--gateway**

Define a gateway for the subnet. If you want to provide a gateway address, you must also provide a
*subnet* option.

**--internal**

Restrict external access of this network

**--ip-range**

Allocate container IP from a range.  The range must be a complete subnet and in CIDR notation.  The *ip-range* option
must be used with a *subnet* option.

**--macvlan**

Create a *Macvlan* based connection rather than a classic bridge.  You must pass an interface name from the host for the
Macvlan connection.

**--subnet**

The subnet in CIDR notation.

## EXAMPLE

Create a network with no options
```
# podman network create
/etc/cni/net.d/cni-podman-4.conflist
```

Create a network named *newnet* that uses *192.5.0.0/16* for its subnet.
```
# podman network create --subnet 192.5.0.0/16 newnet
/etc/cni/net.d/newnet.conflist
```

Create a network named *newnet* that uses *192.168.33.0/24* and defines a gateway as *192.168.133.3*
```
# podman network create --subnet 192.168.33.0/24 --gateway 192.168.33.3 newnet
/etc/cni/net.d/newnet.conflist
```

Create a network that uses a *192.168.55.0/24** subnet and has an IP address range of *192.168.55.129 - 192.168.55.254*.
```
# podman network create --subnet 192.168.55.0/24 --ip-range 192.168.55.128/25
/etc/cni/net.d/cni-podman-5.conflist
```

Create a Macvlan based network using the host interface eth0
```
# podman network create --macvlan eth0 newnet
/etc/cni/net.d/newnet.conflist
```

## SEE ALSO
podman(1), podman-network(1), podman-network-inspect(1)

## HISTORY
August 2019, Originally compiled by Brent Baude <bbaude@redhat.com>
