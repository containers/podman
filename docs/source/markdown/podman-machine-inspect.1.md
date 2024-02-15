% podman-machine-inspect 1

## NAME
podman\-machine\-inspect - Inspect one or more virtual machines

## SYNOPSIS
**podman machine inspect** [*options*] [*name*] ...

## DESCRIPTION

Inspect one or more virtual machines

Obtain greater detail about Podman virtual machines. More than one virtual machine can be
inspected at once.

The default machine name is `podman-machine-default`. If a machine name is not specified as an argument,
then `podman-machine-default` will be inspected.

Rootless only.

## OPTIONS
#### **--format**

Print results with a Go template.

| **Placeholder**     | **Description**                                                       |
| ------------------- | --------------------------------------------------------------------- |
| .ConfigDir ...      | Machine configuration directory location                                   |
| .ConnectionInfo ... | Machine connection information                                        |
| .Created ...        | Machine creation time (string, ISO3601)                               |
| .LastUp ...         | Time when machine was last booted                                     |
| .Name               | Name of the machine                                                   |
| .Resources ...      | Resources used by the machine                                         |
| .Rootful            | Whether the machine prefers rootful or rootless container execution   |
| .Rosetta            | Whether this machine uses Rosetta                               |
| .SSHConfig ...      | SSH configuration info for communicating with machine                 |
| .State              | Machine state                                                         |
| .UserModeNetworking | Whether this machine uses user-mode networking                        |

#### **--help**

Print usage statement.

## EXAMPLES

Inspect the specified Podman machine.
```
$ podman machine inspect podman-machine-default
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine(1)](podman-machine.1.md)**

## HISTORY
April 2022, Originally compiled by Brent Baude <bbaude@redhat.com>
