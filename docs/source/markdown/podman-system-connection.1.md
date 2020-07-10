% podman-system-connection(1)

## NAME
podman\-system\-connection - Record ssh destination for remote podman service

## SYNOPSIS
**podman system connection** [*options*] [*ssh destination*]

## DESCRIPTION
Record ssh destination for remote podman service(s). The ssh destination is given as one of:
 - [user@]hostname[:port]
 - ssh://[user@]hostname[:port]

The user will be prompted for the remote ssh login password or key file pass phrase as required. `ssh-agent` is supported if it is running.

## OPTIONS

**--identity**=*path*

Path to ssh identity file. If the identity file has been encrypted, podman prompts the user for the passphrase.
If no identity file is provided and no user is given, podman defaults to the user running the podman command.
Podman prompts for the login password on the remote server.

**-p**, **--port**=*port*

Port for ssh destination. The default value is `22`.

**--socket-path**=*path*

Path to podman service unix domain socket on the ssh destination host

## EXAMPLE
```
$ podman system connection podman.fubar.com

$ podman system connection --identity ~/.ssh/dev_rsa ssh://root@server.fubar.com:2222

```
## SEE ALSO
podman-system(1) , containers.conf(5) , connections.conf(5)

## HISTORY
June 2020, Originally compiled by Jhon Honce (jhonce at redhat dot com)
