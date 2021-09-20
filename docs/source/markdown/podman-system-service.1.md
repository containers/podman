% podman-service(1)

## NAME
podman\-system\-service - Run an API service

## SYNOPSIS
**podman system service** [*options*]

## DESCRIPTION
The **podman system service** command creates a listening service that will answer API calls for Podman.  You may
optionally provide an endpoint for the API in URI form.  For example, *unix:///tmp/foobar.sock* or *tcp:localhost:8080*.
If no endpoint is provided, defaults will be used.  The default endpoint for a rootfull
service is *unix:///run/podman/podman.sock* and rootless is *unix://$XDG_RUNTIME_DIR/podman/podman.sock* (for
example *unix:///run/user/1000/podman/podman.sock*)

To access the API service inside a container:
- mount the socket as a volume
- run the container with `--security-opt label:disable`

The REST API provided by **podman system service** is split into two parts: a compatibility layer offering support for the Docker v1.40 API, and a Podman-native Libpod layer.
Documentation for the latter is available at *https://docs.podman.io/en/latest/_static/api.html*.
Both APIs are versioned, but the server will not reject requests with an unsupported version set.

Note: The default systemd unit files (system and user) change the log-level option to *info* from *error*. This change provides additional information on each API call.

## OPTIONS

#### **--time**, **-t**

The time until the session expires in _seconds_. The default is 5
seconds. A value of `0` means no timeout, therefore the session will not expire.

The default timeout can be changed via the `service_timeout=VALUE` field in containers.conf.
See **[containers.conf(5)](https://github.com/containers/common/blob/master/docs/containers.conf.5.md)** for more information.

#### **--cors**

CORS headers to inject to the HTTP response. The default value is empty string which disables CORS headers.

#### **--help**, **-h**

Print usage statement.

## EXAMPLES

Run an API listening for 5 seconds using the default socket.
```
podman system service --time 5
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-system-connection(1)](podman-system-connection.1.md)**, **[containers.conf(5)](https://github.com/containers/common/blob/master/docs/containers.conf.5.md)**

## HISTORY
January 2020, Originally compiled by Brent Baude `<bbaude@redhat.com>`
November 2020, Updated by Jhon Honce (jhonce at redhat dot com)
