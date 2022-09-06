% podman-machine-info 1

## NAME
podman\-machine\-info - Display machine host info

## SYNOPSIS
**podman machine info**

## DESCRIPTION

Display information pertaining to the machine host.
Rootless only, as all `podman machine` commands can be only be used with rootless Podman.

## OPTIONS

#### **--format**, **-f**=*format*

Change output format to "json" or a Go template.

#### **--help**

Print usage statement.

## EXAMPLES

```
$ podman machine info
$ podman machine info --format json
$ podman machine info --format {{.Host.Arch}}
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine(1)](podman-machine.1.md)**

## HISTORY
June 2022, Originally compiled by Ashley Cui <acui@redhat.com>
