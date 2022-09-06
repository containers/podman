% podman-network-ls 1

## NAME
podman\-network\-ls - Display a summary of networks

## SYNOPSIS
**podman network ls**  [*options*]

## DESCRIPTION
Displays a list of existing podman networks.

## OPTIONS
#### **--filter**, **-f**=*filter=value*

Provide filter values.

The *filters* argument format is of `key=value`. If there is more than one *filter*, then pass multiple OPTIONS: **--filter** *foo=bar* **--filter** *bif=baz*.

Supported filters:

| **Filter** | **Description**                                                                                  |
| ---------- | ------------------------------------------------------------------------------------------------ |
| driver     | Filter by driver type.                                                                           |
| id         | Filter by full or partial network ID.                                                            |
| label      | Filter by network with (or without, in the case of label!=[...] is used) the specified labels.   |
| name       | Filter by network name (accepts `regex`).                                                        |
| until      | Filter by networks created before given timestamp.                                               |
| dangling   | Filter by networks with no containers attached.                                                  |


The `driver` filter accepts values: `bridge`, `macvlan`, `ipvlan`.

The `label` *filter* accepts two formats. One is the `label`=*key* or `label`=*key*=*value*, which shows images with the specified labels. The other format is the `label!`=*key* or `label!`=*key*=*value*, which shows images without the specified labels.

The `until` *filter* can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. 10m, 1h30m) computed relative to the machine’s time.

The `dangling` *filter* accepts values `true` or `false`.

#### **--format**=*format*

Change the default output format.  This can be of a supported type like 'json'
or a Go template.
Valid placeholders for the Go template are listed below:

| **Placeholder**   | **Description**                           |
| ----------------- | ----------------------------------------- |
| .ID               | Network ID                                |
| .Name             | Network name                              |
| .Driver           | Network driver                            |
| .Labels           | Network labels                            |
| .Options          | Network options                           |
| .IPAMOptions      | Network ipam options                      |
| .Created          | Timestamp when the network was created    |
| .Internal         | Network is internal (boolean)             |
| .IPv6Enabled      | Network has ipv6 subnet (boolean)         |
| .DNSEnabled       | Network has dns enabled (boolean)         |
| .NetworkInterface | Name of the network interface on the host |
| .Subnets          | List of subnets on this network           |

#### **--no-trunc**

Do not truncate the network ID.

#### **--noheading**

Omit the table headings from the listing of networks.

#### **--quiet**, **-q**

The `quiet` option will restrict the output to only the network names.

## EXAMPLE

Display networks

```
$ podman network ls
NETWORK ID    NAME         DRIVER
88a7120ee19d  podman       bridge
6dd508dbf8cd  podman6  bridge
8e35c2cd3bf6  podman5  macvlan
```

Display only network names
```
$ podman network ls -q
podman
podman2
outside
podman9
```

Display name of network which support bridge plugin
```
$ podman network ls --filter driver=bridge --format {{.Name}}
podman
podman2
podman9
```
List networks with their subnets
```
$ podman network ls --format "{{.Name}}: {{range .Subnets}}{{.Subnet}} {{end}}"
podman: 10.88.0.0/16
podman3: 10.89.30.0/24 fde4:f86f:4aab:e68f::/64
macvlan:
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-network(1)](podman-network.1.md)**, **[podman-network-inspect(1)](podman-network-inspect.1.md)**, **[podman-network-create(1)](podman-network-create.1.md)**

## HISTORY
August 2021, Updated with the new network format by Paul Holzinger <pholzing@redhat.com>

August 2019, Originally compiled by Brent Baude <bbaude@redhat.com>
