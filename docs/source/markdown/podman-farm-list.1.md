% podman-farm-list 1

## NAME
podman\-farm\-list - List the existing farms

## SYNOPSIS
**podman farm list** [*options*]

**podman farm ls** [*options*]

## DESCRIPTION
List all the existing farms.

## OPTIONS

#### **--format**=*format*

Change the default output format.  This can be of a supported type like 'json' or a Go template.
Valid placeholders for the Go template listed below:

| **Placeholder** | **Description**                                                       |
| --------------- | --------------------------------------------------------------------- |
| .Connections    | List of all system connections in the farm                            |
| .Default        | Indicates whether farm is the default                                 |
| .Name           | Farm name                                                             |
| .ReadWrite      | Indicates if this farm can be modified using the podman farm commands |

## EXAMPLE

List all farms:
```
$ podman farm list
Name        Connections  Default     ReadWrite
farm1       [f38 f37]    false       true
farm2       [f37]        true        true
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-farm(1)](podman-farm.1.md)**

## HISTORY
July 2023, Originally compiled by Urvashi Mohnani (umohnani at redhat dot com)
