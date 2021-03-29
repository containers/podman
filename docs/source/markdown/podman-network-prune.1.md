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
#### **\-\-force**, **-f**

Do not prompt for confirmation

## EXAMPLE
Prune networks

```
podman network prune
```


## SEE ALSO
podman(1), podman-network(1), podman-network-remove(1)

## HISTORY
February 2021, Originally compiled by Brent Baude <bbaude@redhat.com>
