% podman-system-connection-add 1

## NAME
podman\-system\-connection\-add - Record destination for the Podman service

## SYNOPSIS
**podman system connection add** [*options*] *name* *destination*

## DESCRIPTION
Record ssh destination for remote podman service(s). The ssh destination is given as one of:
 - [user@]hostname[:port]
 - ssh://[user@]hostname[:port]
 - unix://path
 - tcp://hostname:port

The user is prompted for the remote ssh login password or key file passphrase as required. The `ssh-agent` is supported if it is running.

## OPTIONS

#### **--default**, **-d**

Make the new destination the default for this user. The default is **false**.

#### **--identity**=*path*

Path to ssh identity file. If the identity file has been encrypted, Podman prompts the user for the passphrase.
If no identity file is provided and no user is given, Podman defaults to the user running the podman command.
Podman prompts for the login password on the remote server.

#### **--port**, **-p**=*port*

Port for ssh destination. The default value is `22`.

#### **--socket-path**=*path*

Path to the Podman service unix domain socket on the ssh destination host

## EXAMPLE

Add a named system connection:
```
$ podman system connection add QA podman.example.com
```

Add a system connection using SSH data:
```
$ podman system connection add --identity ~/.ssh/dev_rsa production ssh://root@server.example.com:2222
```

Add a named system connection to local Unix domain socket:
```
$ podman system connection add testing unix:///run/podman/podman.sock
```

Add a named system connection to local tcp socket:
```
$ podman system connection add debug tcp://localhost:8080
```
## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-system(1)](podman-system.1.md)**, **[podman-system-connection(1)](podman-system-connection.1.md)**


## HISTORY
June 2020, Originally compiled by Jhon Honce (jhonce at redhat dot com)
