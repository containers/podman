% podman-network-ls(1)

## NAME
podman\-network\-ls - Display a summary of CNI networks

## SYNOPSIS
**podman network ls**  [*options*]

## DESCRIPTION
Displays a list of existing podman networks.

## OPTIONS
#### **--filter**, **-f**

Filter output based on conditions given.
Multiple filters can be given with multiple uses of the --filter option.
Filters with the same key work inclusive with the only exception being
`label` which is exclusive. Filters with different keys always work exclusive.

Valid filters are listed below:

| **Filter** | **Description**                                                                       |
| ---------- | ------------------------------------------------------------------------------------- |
| name       | [Name] Network name (accepts regex)                                                   |
| id         | [ID] Full or partial network ID                                                       |
| label      | [Key] or [Key=Value] Label assigned to a network                                      |
| plugin     | [Plugin] CNI plugins included in a network (e.g `bridge`,`portmap`,`firewall`,`tuning`,`dnsname`,`macvlan`) |
| driver     | [Driver] Only `bridge` is supported                                                   |

#### **--format**

Change the default output format.  This can be of a supported type like 'json'
or a Go template.
Valid placeholders for the Go template are listed below:

| **Placeholder** | **Description**                 |
| --------------- | --------------------------------|
| .ID             | Network ID                      |
| .Name           | Network name                    |
| .Plugins        | Network Plugins                 |
| .Labels         | Network labels                  |
| .Version        | CNI Version of the config file	|

#### **--noheading**

Omit the table headings from the listing of networks.

#### **--no-trunc**

Do not truncate the network ID. The network ID is not displayed by default and must be specified with **--format**.

#### **--quiet**, **-q**

The `quiet` option will restrict the output to only the network names.

## EXAMPLE

Display networks

```
# podman network ls
NAME      VERSION   PLUGINS
podman    0.3.0     bridge,portmap
podman2   0.3.0     bridge,portmap
outside   0.3.0     bridge
podman9   0.3.0     bridge,portmap
```

Display only network names
```
# podman network ls -q
podman
podman2
outside
podman9
```

Display name of network which support bridge plugin
```
# podman network ls --filter plugin=portmap --format {{.Name}}
podman
podman2
podman9
```

## SEE ALSO
podman(1), podman-network(1), podman-network-inspect(1)

## HISTORY
August 2019, Originally compiled by Brent Baude <bbaude@redhat.com>
