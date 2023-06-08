% podman-network-create 1

## NAME
podman\-network-create - Create a Podman network

## SYNOPSIS
**podman network create**  [*options*] [*name*]

## DESCRIPTION
Create a network configuration for use with Podman. By default, Podman creates a bridge connection.
A *Macvlan* connection can be created with the *-d macvlan* option. A parent device for macvlan or
ipvlan can be designated with the *-o parent=`<device>`* or *--network-interface=`<device>`* option.

If no options are provided, Podman assigns a free subnet and name for the network.

Upon completion of creating the network, Podman displays the name of the newly added network.

NOTE: The support for the network name "pasta" is deprecated and will be removed in the next major
release because it is used as a special network mode in **podman run/create --network**.

## OPTIONS
#### **--disable-dns**

Disables the DNS plugin for this network which if enabled, can perform container to container name
resolution. It is only supported with the `bridge` driver, for other drivers it is always disabled.

#### **--dns**=*ip*

Set network-scoped DNS resolver/nameserver for containers in this network. If not set, the host servers from `/etc/resolv.conf` is used.  It can be overwritten on the container level with the `podman run/create --dns` option. This option can be specified multiple times to set more than one IP.

#### **--driver**, **-d**=*driver*

Driver to manage the network. Currently `bridge`, `macvlan` and `ipvlan` are supported. Defaults to `bridge`.
As rootless the `macvlan` and `ipvlan` driver have no access to the host network interfaces because rootless networking requires a separate network namespace.

The netavark backend allows the use of so called *netavark plugins*, see the
[plugin-API.md](https://github.com/containers/netavark/blob/main/plugin-API.md)
documentation in netavark. The binary must be placed in a specified directory
so podman can discover it, this list is set in `netavark_plugin_dirs` in
**[containers.conf(5)](https://github.com/containers/common/blob/main/docs/containers.conf.5.md)**
under the `[network]` section.

The name of the plugin can then be used as driver to create a network for your plugin.
The list of all supported drivers and plugins can be seen with `podman info --format {{.Plugins.Network}}`.

#### **--gateway**=*ip*

Define a gateway for the subnet. To provide a gateway address, a
*subnet* option is required. Can be specified multiple times.
The argument order of the **--subnet**, **--gateway** and **--ip-range** options must match.

#### **--ignore**

Ignore the create request if a network with the same name already exists instead of failing.
Note, trying to create a network with an existing name and different parameters does not change the configuration of the existing one.

#### **--interface-name**=*name*

This option maps the the *network_interface* option in the network config, see **podman network inspect**.
Depending on the driver, this can have different effects; for `bridge`, it uses the bridge interface name.
For `macvlan` and `ipvlan`, it is the parent device on the host. It is the same as `--opt parent=...`.

#### **--internal**

Restrict external access of this network. Note when using this option, the dnsname plugin is automatically disabled.

#### **--ip-range**=*range*

Allocate container IP from a range.  The range must be a complete subnet and in CIDR notation.  The *ip-range* option
must be used with a *subnet* option. Can be specified multiple times.
The argument order of the **--subnet**, **--gateway** and **--ip-range** options must match.

#### **--ipam-driver**=*driver*

Set the ipam driver (IP Address Management Driver) for the network. When unset podman chooses an
ipam driver automatically based on the network driver.

Valid values are:

 - `dhcp`: IP addresses are assigned from a dhcp server on the network. This driver is not yet supported with netavark. For CNI the *dhcp* plugin needs to be activated before.
 - `host-local`: IP addresses are assigned locally.
 - `none`: No ip addresses are assigned to the interfaces.

View the driver in the **podman network inspect** output under the `ipam_options` field.

#### **--ipv6**

Enable IPv6 (Dual Stack) networking. If no subnets are given, it allocates an ipv4 and an ipv6 subnet.

#### **--label**=*label*

Set metadata for a network (e.g., --label mykey=value).

#### **--opt**, **-o**=*option*

Set driver specific options.

All drivers accept the `mtu` and `metric` options.

- `mtu`: Sets the Maximum Transmission Unit (MTU) and takes an integer value.
- `metric` Sets the Route Metric for the default route created in every container joined to this network. Accepts a positive integer value. Can only be used with the Netavark network backend.

Additionally the `bridge` driver supports the following options:

- `vlan`: This option assign VLAN tag and enables vlan\_filtering. Defaults to none.
- `isolate`: This option isolates networks by blocking traffic between those that have this option enabled.
- `com.docker.network.bridge.name`: This option assigns the given name to the created Linux Bridge
- `com.docker.network.driver.mtu`: Sets the Maximum Transmission Unit (MTU) and takes an integer value.

The `macvlan` and `ipvlan` driver support the following options:

- `parent`: The host device which is used for the macvlan interface. Defaults to the default route interface.
- `mode`: This option sets the specified ip/macvlan mode on the interface.
  - Supported values for `macvlan` are `bridge`, `private`, `vepa`, `passthru`. Defaults to `bridge`.
  - Supported values for `ipvlan` are `l2`, `l3`, `l3s`. Defaults to `l2`.

#### **--subnet**=*subnet*

The subnet in CIDR notation. Can be specified multiple times to allocate more than one subnet for this network.
The argument order of the **--subnet**, **--gateway** and **--ip-range** options must match.
This is useful to set a static ipv4 and ipv6 subnet.

## EXAMPLE

Create a network with no options.
```
$ podman network create
podman2
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

Create a network that uses a *192.168.55.0/24* subnet and has an IP address range of *192.168.55.129 - 192.168.55.254*.
```
$ podman network create --subnet 192.168.55.0/24 --ip-range 192.168.55.128/25
podman5
```

Create a network with a static ipv4 and ipv6 subnet and set a gateway.
```
$ podman network create --subnet 192.168.55.0/24 --gateway 192.168.55.3 --subnet fd52:2a5a:747e:3acd::/64 --gateway fd52:2a5a:747e:3acd::10
podman4
```

Create a Macvlan based network using the host interface eth0. Macvlan networks can only be used as root.
```
$ sudo podman network create -d macvlan -o parent=eth0 --subnet 192.5.0.0/16 newnet
newnet
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-network(1)](podman-network.1.md)**, **[podman-network-inspect(1)](podman-network-inspect.1.md)**, **[podman-network-ls(1)](podman-network-ls.1.md)**, **[containers.conf(5)](https://github.com/containers/common/blob/main/docs/containers.conf.5.md)**

## HISTORY
August 2021, Updated with the new network format by Paul Holzinger <pholzing@redhat.com>

August 2019, Originally compiled by Brent Baude <bbaude@redhat.com>
