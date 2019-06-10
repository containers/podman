% podman-remote.conf(5)

## NAME
podman-remote.conf - configuration file for the podman remote client

## DESCRIPTION
The libpod.conf file is the default configuration file for all tools using
libpod to manage containers.

The podman-remote.conf file is the default configuration file for the podman
remote client.  It is in the TOML format.  It is primarily used to keep track
of the user's remote connections.

## CONNECTION OPTIONS
**destination** = ""
  The hostname or IP address of the remote system

**username** = ""
  The username to use when connecting to the remote system

**default** = bool
  Denotes whether the connection is the default connection for the user.  The default connection
  is used when the user does not specify a destination or connection name to `podman`.


## EXAMPLE

The following example depicts a configuration file with two connections.  One of the connections
is designated as the default connection.
```
[connections]
    [connections.host1]
    destination = "host1"
    username = "homer"
    default = true

    [connections.host2]
    destination = "192.168.122.133"
    username = "fedora"
```

## FILES
  `/$HOME/.config/containers/podman-remote.conf`, default location for the podman remote
configuration file

## HISTORY
May 2019, Originally compiled by Brent Baude<bbaude@redhat.com>
