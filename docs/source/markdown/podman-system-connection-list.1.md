% podman-system-connection-list(1)

## NAME
podman\-system\-connection\-list - List the destination for the Podman service(s)

## SYNOPSIS
**podman system connection list** [*options*]

**podman system connection ls** [*options*]

## DESCRIPTION
List ssh destination(s) for podman service(s).

## OPTIONS

#### **--format**=*format*

Change the default output format.  This can be of a supported type like 'json' or a Go template.
Valid placeholders for the Go template listed below:

| **Placeholder** | **Description**                                                               |
| --------------- | ----------------------------------------------------------------------------- |
| *.Name*         | Connection Name/Identifier |
| *.Identity*     | Path to file containing SSH identity |
| *.URI*          | URI to podman service. Valid schemes are ssh://[user@]*host*[:port]*Unix domain socket*[?secure=True], unix://*Unix domain socket*, and tcp://localhost[:*port*] |

An asterisk is appended to the default connection.

## EXAMPLE
```
$ podman system connection list
Name URI                                           Identity
devl ssh://root@example.com/run/podman/podman.sock ~/.ssh/id_rsa
```
## SEE ALSO
podman-system(1) , containers.conf(5)

## HISTORY
July 2020, Originally compiled by Jhon Honce (jhonce at redhat dot com)
