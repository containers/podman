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

Upon completion of creating the network, Podman will display the name of the newly added network.

## OPTIONS
#### **--disable-dns**

Disables the DNS plugin for this network which if enabled, can perform container to container name
resolution.

#### **--driver**, **-d**

Driver to manage the network. Currently `bridge`, `macvlan` and `ipvlan` are supported. Defaults to `bridge`.
As rootless the `macvlan` and `ipvlan` driver have no access to the host network interfaces because rootless networking requires a separate network namespace.

#### **--opt**=*option*, **-o**

Set driver specific options.

All drivers accept the `mtu` option. The `mtu` option sets the Maximum Transmission Unit (MTU) and takes an integer value.

Additionally the `bridge` driver supports the following option:
- `vlan`: This option assign VLAN tag and enables vlan\_filtering. Defaults to none.

The `macvlan` and `ipvlan` driver support the following options:
- `parent`: The host device which should be used for the macvlan interface. Defaults to the default route interface.
- `mode`: This options sets the specified ip/macvlan mode on the interface.
  - Supported values for `macvlan` are `bridge`, `private`, `vepa`, `passthru`. Defaults to `bridge`.
  - Supported values for `ipvlan` are `l2`, `l3`, `l3s`. Defaults to `l2`.

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

#### **--subnet**

The subnet in CIDR notation.

#### **--ipv6**

Enable IPv6 (Dual Stack) networking.

## EXAMPLE

Create a network with no options.
```
$ podman network create
cni-podman2
```

Create a network named *newnet* that uses *192.5.0.0/16* for its subnet.
```
$ podman network create --subnet 192.5.0.0/16 newnet
newnet
```

Create an IPv6 network named *newnetv6* with a subnet of *2001:db8::/64*.
```
$ podman network create --subnet 2001:db8::/64 --ipv6 newnetv6
newnetv6
```

Create a network named *newnet* that uses *192.168.33.0/24* and defines a gateway as *192.168.133.3*.
```
$ podman network create --subnet 192.168.33.0/24 --gateway 192.168.33.3 newnet
newnet
```

Create a network that uses a *192.168.55.0/24** subnet and has an IP address range of *192.168.55.129 - 192.168.55.254*.
```
$ podman network create --subnet 192.168.55.0/24 --ip-range 192.168.55.128/25
cni-podman5
```

Create a Macvlan based network using the host interface eth0. Macvlan networks can only be used as root.
```
# podman network create -d macvlan -o parent=eth0 newnet
newnet
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-network(1)](podman-network.1.md)**, **[podman-network-inspect(1)](podman-network-inspect.1.md)**, **[podman-network-ls(1)](podman-network-ls.1.md)**

## HISTORY
August 2021, Updated with the new network format by Paul Holzinger <pholzing@redhat.com>

August 2019, Originally compiled by Brent Baude <bbaude@redhat.com>
