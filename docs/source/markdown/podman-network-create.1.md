% podman-network-create(1)

## NAME
podman\-network-create - Create a Podman CNI network

## SYNOPSIS
**podman network create**  [*options*] name

## DESCRIPTION
Create a CNI-network configuration for use with Podman. By default, Podman creates a bridge connection.
A *Macvlan* connection can be created with the *-d macvlan* option. A parent device for macvlan can
be designated with the *-o parent=`<device>`* option. In the case of *Macvlan* connections, the
CNI *dhcp* plugin needs to be activated or the container image must have a DHCP client to interact
with the host network's DHCP server.

If no options are provided, Podman will assign a free subnet and name for your network.

Upon completion of creating the network, Podman will display the path to the newly added network file.

## OPTIONS
#### **--disable-dns**

Disables the DNS plugin for this network which if enabled, can perform container to container name
resolution.

#### **--driver**, **-d**

Driver to manage the network (default "bridge").  Currently only `bridge` is supported.

#### **--opt**=*option*, **-o**

Set driver specific options.

For the `bridge` driver the following options are supported: `mtu` and `vlan`.
The `mtu` option sets the Maximum Transmission Unit (MTU) and takes an integer value.
The `vlan` option assign VLAN tag and enables vlan\_filtering. Defaults to none.

#### **--gateway**

Define a gateway for the subnet. If you want to provide a gateway address, you must also provide a
*subnet* option.

#### **--internal**

Restrict external access of this network. Note when using this option, the dnsname plugin will be
automatically disabled.

#### **--ip-range**

Allocate container IP from a range.  The range must be a complete subnet and in CIDR notation.  The *ip-range* option
must be used with a *subnet* option.

#### **--label**

Set metadata for a network (e.g., --label mykey=value).

#### **--macvlan**

*This option is being deprecated*

Create a *Macvlan* based connection rather than a classic bridge.  You must pass an interface name from the host for the
Macvlan connection.

#### **--subnet**

The subnet in CIDR notation.

#### **--ipv6**

Enable IPv6 (Dual Stack) networking. You must pass a IPv6 subnet. The *subnet* option must be used with the *ipv6* option.

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

Create an IPv6 network named *newnetv6*, you must specify the subnet for this network, otherwise the command will fail.
For this example, we use *2001:db8::/64* for its subnet.
```
# podman network create --subnet 2001:db8::/64 --ipv6 newnetv6
/etc/cni/net.d/newnetv6.conflist
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
# podman network create -d macvlan -o parent=eth0 newnet
/etc/cni/net.d/newnet.conflist
```

## SEE ALSO
podman(1), podman-network(1), podman-network-inspect(1)

## HISTORY
August 2019, Originally compiled by Brent Baude <bbaude@redhat.com>
