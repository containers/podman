% podman-volume-prune(1)

## NAME
podman\-volume\-prune - Remove all unused volumes

## SYNOPSIS
**podman volume prune** [*options*]

## DESCRIPTION

Removes unused volumes. By default all unused volumes will be removed, the **--filter** flag can
be used to filter specific volumes. You will be prompted to confirm the removal of all the
unused volumes. To bypass the confirmation, use the **--force** flag.


## OPTIONS

#### **--filter**

Provide filter values.

The *filters* argument format is of `key=value`. If there is more than one *filter*, then pass multiple OPTIONS: **--filter** *foo=bar* **--filter** *bif=baz*.

Supported filters:

| Filter             | Description                                                                 |
| :----------------: | --------------------------------------------------------------------------- |
| *label*            | Only remove volumes, with (or without, in the case of label!=[...] is used) the specified labels.                  |
| *until*            | Only remove volumes created before given timestamp.           |

The `label` *filter* accepts two formats. One is the `label`=*key* or `label`=*key*=*value*, which removes volumes with the specified labels. The other format is the `label!`=*key* or `label!`=*key*=*value*, which removes volumes without the specified labels.

The `until` *filter* can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. 10m, 1h30m) computed relative to the machine’s time.

#### **--force**, **-f**

Do not prompt for confirmation.

#### **--help**

Print usage statement


## EXAMPLES

```
$ podman volume prune

$ podman volume prune --force

$ podman volume prune --filter label=mylabel=mylabelvalue
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-volume(1)](podman-volume.1.md)**

## HISTORY
November 2018, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
