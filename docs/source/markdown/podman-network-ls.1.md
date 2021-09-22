% podman-network-ls(1)

## NAME
podman\-network\-ls - Display a summary of CNI networks

## SYNOPSIS
**podman network ls**  [*options*]

## DESCRIPTION
Displays a list of existing podman networks.

## OPTIONS
#### **--filter**, **-f**=*filter=value*

Filter output based on conditions given.
Multiple filters can be given with multiple uses of the --filter option.
Filters with the same key work inclusive with the only exception being
`label` which is exclusive. Filters with different keys always work exclusive.

Valid filters are listed below:

| **Filter** | **Description**                                                   |
| ---------- | ----------------------------------------------------------------- |
| name       | [Name] Network name (accepts regex)                               |
| id         | [ID] Full or partial network ID                                   |
| label      | [Key] or [Key=Value] Label assigned to a network                  |
| driver     | [Driver] `bridge` or ,`macvlan` is supported                      |
| until      | [Until] Show all networks that were created before the given time |

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

#### **--noheading**

Omit the table headings from the listing of networks.

#### **--no-trunc**

Do not truncate the network ID.

#### **--quiet**, **-q**

The `quiet` option will restrict the output to only the network names.

## EXAMPLE

Display networks

```
$ podman network ls
NETWORK ID    NAME         DRIVER
88a7120ee19d  podman       bridge
6dd508dbf8cd  cni-podman6  bridge
8e35c2cd3bf6  cni-podman5  macvlan
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
cni-podman3: 10.89.30.0/24 fde4:f86f:4aab:e68f::/64
macvlan:
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-network(1)](podman-network.1.md)**, **[podman-network-inspect(1)](podman-network-inspect.1.md)**, **[podman-network-create(1)](podman-network-create.1.md)**

## HISTORY
August 2021, Updated with the new network format by Paul Holzinger <pholzing@redhat.com>

August 2019, Originally compiled by Brent Baude <bbaude@redhat.com>
