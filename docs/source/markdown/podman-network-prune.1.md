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

Provide filter values.

The *filters* argument format is of `key=value`. If there is more than one *filter*, then pass multiple OPTIONS: **--filter** *foo=bar* **--filter** *bif=baz*.

Supported filters:

| Filter             | Description                                                                 |
| :----------------: | --------------------------------------------------------------------------- |
| *until*            | Only remove networks created before given timestamp.           |
| *label*            | Only remove networks, with (or without, in the case of label!=[...] is used) the specified labels.                  |

The `until` *filter* can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. 10m, 1h30m) computed relative to the machine’s time.

The `label` *filter* accepts two formats. One is the `label`=*key* or `label`=*key*=*value*, which removes networks with the specified labels. The other format is the `label!`=*key* or `label!`=*key*=*value*, which removes networks without the specified labels.

## EXAMPLE
Prune networks

```
podman network prune
```


## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-network(1)](podman-network.1.md)**, **[podman-network-rm(1)](podman-network-rm.1.md)**

## HISTORY
February 2021, Originally compiled by Brent Baude <bbaude@redhat.com>
