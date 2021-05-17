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

- dangling
- driver
- label
- name
- opt
- scope

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
podman-volume(1)

## HISTORY
November 2018, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
