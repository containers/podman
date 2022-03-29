% podman-volume-ls(1)

## NAME
podman\-volume\-ls - List all the available volumes

## SYNOPSIS
**podman volume ls** [*options*]

## DESCRIPTION

Lists all the volumes that exist. The output can be filtered using the **--filter**
flag and can be formatted to either JSON or a Go template using the **--format**
flag. Use the **--quiet** flag to print only the volume names.

## OPTIONS

#### **--filter**=*filter*, **-f**

Volumes can be filtered by the following attributes:

| **Filter** | **Description**                                                                       |
| ---------- | ------------------------------------------------------------------------------------- |
| dangling   | [Dangling] Matches all volumes not referenced by any containers                       |
| driver     | [Driver] Matches volumes based on their driver                                        |
| label      | [Key] or [Key=Value] Label assigned to a volume                                       |
| name       | [Name] Volume name (accepts regex)                                                    |
| opt        | Matches a storage driver options                                                      |
| scope      | Filters volume by scope                                                               |
| until      | Only remove volumes created before given timestamp                                    |

#### **--format**=*format*

Format volume output using Go template.

#### **--help**

Print usage statement.

#### **--noheading**

Omit the table headings from the listing of volumes.

#### **--quiet**, **-q**

Print volume output in quiet mode. Only print the volume names.

## EXAMPLES

```
$ podman volume ls

$ podman volume ls --format json

$ podman volume ls --format "{{.Driver}} {{.Scope}}"

$ podman volume ls --filter name=foo,label=blue

$ podman volume ls --filter label=key=value
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-volume(1)](podman-volume.1.md)**

## HISTORY
November 2018, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
