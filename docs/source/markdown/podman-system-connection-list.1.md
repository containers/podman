% podman-system-connection-list 1

## NAME
podman\-system\-connection\-list - List the destination for the Podman service(s)

## SYNOPSIS
**podman system connection list** [*options*]

**podman system connection ls** [*options*]

## DESCRIPTION
List ssh destination(s) for podman service(s).

## OPTIONS

#### **--format**, **-f**=*format*

Change the default output format.  This can be of a supported type like 'json' or a Go template.
Valid placeholders for the Go template listed below:

| **Placeholder** | **Description**                                                               |
| --------------- | ----------------------------------------------------------------------------- |
| .Default        | Indicates whether connection is the default |
| .Identity       | Path to file containing SSH identity |
| .Name           | Connection Name/Identifier |
| .ReadWrite      | Indicates if this connection can be modified using the system connection commands |
| .URI            | URI to podman service. Valid schemes are ssh://[user@]*host*[:port]*Unix domain socket*[?secure=True], unix://*Unix domain socket*, and tcp://localhost[:*port*] |

#### **--quiet**, **-q**

Only show connection names

## EXAMPLE

List system connections:
```
$ podman system connection list
Name URI                                                      Identity	    Default  ReadWrite
deva ssh://root@example.com:/run/podman/podman.sock           ~/.ssh/id_rsa true     true
devb ssh://user@example.com:/run/user/1000/podman/podman.sock ~/.ssh/id_rsa false    true
```
## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-system(1)](podman-system.1.md)**, **[podman-system-connection(1)](podman-system-connection.1.md)**

## HISTORY
July 2020, Originally compiled by Jhon Honce (jhonce at redhat dot com)
