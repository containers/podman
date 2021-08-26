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

#### **--force**, **-f**

Do not prompt for confirmation.

#### **--filter**

Filter volumes to be pruned. Volumes can be filtered by the following attributes:

| **Filter** | **Description**                                                                       |
| ---------- | ------------------------------------------------------------------------------------- |
| label      | [Key] or [Key=Value] Label assigned to a volume                                       |
| until      | Only remove volumes created before given timestamp                                    |

#### **--help**

Print usage statement


## EXAMPLES

```
$ podman volume prune

$ podman volume prune --force

$ podman volume prune --filter label=mylabel=mylabelvalue
```

## SEE ALSO
podman-volume(1)

## HISTORY
November 2018, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
