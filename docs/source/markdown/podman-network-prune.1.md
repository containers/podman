% podman-network-prune(1)

## NAME
podman\-network\-prune - Remove all unused networks

## SYNOPSIS
**podman network prune** [*options*]

## DESCRIPTION
Remove all unused networks.  An unused network is defined by a network which
has no containers connected or configured to connect to it. It will not remove
the so-called default network which goes by the name of *podman*.

## OPTIONS
#### **--force**, **-f**

Do not prompt for confirmation

#### **--filter**

Filter output based on conditions given.
Multiple filters can be given with multiple uses of the --filter option.
Filters with the same key work inclusive with the only exception being
`label` which is exclusive. Filters with different keys always work exclusive.

Valid filters are listed below:

| **Filter** | **Description**                                                                       |
| ---------- | ------------------------------------------------------------------------------------- |
| label      | [Key] or [Key=Value] Label assigned to a network                                      |
| until      | only remove networks created before given timestamp                                   |

## EXAMPLE
Prune networks

```
podman network prune
```


## SEE ALSO
podman(1), podman-network(1), podman-network-remove(1)

## HISTORY
February 2021, Originally compiled by Brent Baude <bbaude@redhat.com>
