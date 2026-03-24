% podman-volume-prune 1

## NAME
podman\-volume\-prune - Remove unused volumes

## SYNOPSIS
**podman volume prune** [*options*]

## DESCRIPTION

Remove unused volumes. By default only **anonymous** (unnamed) unused volumes are removed.
Anonymous volumes are those created with a container (e.g. **podman run -v /path** without a volume name).
Use **--all** (or **-a**) to remove all unused volumes, including named ones.

The **--filter** flag can be used to restrict which volumes are considered. Users are prompted to confirm
removal unless **--force** is used.

## OPTIONS

#### **--all**, **-a**

Remove all unused volumes (anonymous and named). Without this option, only anonymous unused volumes are removed.

#### **--filter**

Provide filter values.

The *filters* argument format is of `key=value`. If there is more than one *filter*, then pass multiple OPTIONS: **--filter** *foo=bar* **--filter** *bif=baz*.

Supported filters:

| Filter      | Description                                                                                                |
|:-----------:|------------------------------------------------------------------------------------------------------------|
| all         | [Bool] When true, remove all unused volumes (same as **--all**). When false or unset, only anonymous volumes are considered. |
| anonymous   | [Bool] Only remove volumes that are anonymous (true) or named (false).                                     |
| label       | [String] Only remove volumes, with (or without, in the case of label!=[...] is used) the specified labels. |
| label!      | [String] Only remove volumes without the specified labels.                                                 |
| until       | [DateTime] Only remove volumes created before given timestamp.                                             |
| after/since | [Volume] Filter by volumes created after the given VOLUME (name or tag)                                    |

The `label` *filter* accepts two formats. One is the `label`=*key* or `label`=*key*=*value*, which removes volumes with the specified labels. The other format is the `label!`=*key* or `label!`=*key*=*value*, which removes volumes without the specified labels.

The `until` *filter* can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. 10m, 1h30m) computed relative to the machine's time.

#### **--force**, **-f**

Do not prompt for confirmation.

#### **--help**

Print usage statement


## EXAMPLES

Prune only anonymous unused volumes (default).
```
$ podman volume prune
```

Prune only anonymous unused volumes without confirmation.
```
$ podman volume prune --force
```

Prune all unused volumes (anonymous and named).
```
$ podman volume prune --all --force
```

Prune all unused volumes using the filter (equivalent to **--all**).
```
$ podman volume prune --filter all=true --force
```

Prune unused volumes that have the specified label.
```
$ podman volume prune --filter label=mylabel=mylabelvalue
```

Prune unused volumes that do NOT have a specific label key/value:
```
$ podman volume prune --filter label!=mylabel=mylabelvalue
```

Prune unused volumes that have a specific label key (regardless of value):
```
$ podman volume prune --filter label=environment
```

Prune unused volumes that do NOT have a specific label key:
```
$ podman volume prune --filter label!=environment
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-volume(1)](podman-volume.1.md)**

## HISTORY
November 2018, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
