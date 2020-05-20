% podman-network-ls(1)

## NAME
podman\-network\-ls - Display a summary of CNI networks

## SYNOPSIS
**podman network ls**  [*options*]

## DESCRIPTION
Displays a list of existing podman networks. This command is not available for rootless users.

## OPTIONS
**--quiet**, **-q**

The `quiet` option will restrict the output to only the network names.

**--format**, **-f**

Pretty-print networks to JSON or using a Go template.

**--filter**

Provide filter values (e.g. 'name=podman').

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
