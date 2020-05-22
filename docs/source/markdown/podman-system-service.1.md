% podman-service(1)

## NAME
podman\-system\-service - Run an API service

## SYNOPSIS
**podman system service** [*options*]

## DESCRIPTION
The **podman system service** command creates a listening service that will answer API calls for Podman.  You may
optionally provide an endpoint for the API in URI form.  For example, *unix://tmp/foobar.sock* or *tcp:localhost:8080*.
If no endpoint is provided, defaults will be used.  The default endpoint for a rootfull
service is *unix:/run/podman/podman.sock* and rootless is *unix:/$XDG_RUNTIME_DIR/podman/podman.sock* (for
example *unix:/run/user/1000/podman/podman.sock*)

## OPTIONS

**--time**, **-t**

The time until the session expires in _milliseconds_. The default is 1
second. A value of `0` means no timeout and the session will not expire.

**--help**, **-h**

Print usage statement.

## EXAMPLES

Run an API listening for 5 seconds using the default socket.
```
podman system service --timeout 5000
```

Run the podman varlink service with an alternate URI and accept the default timeout.
```
$ podman system service --varlink unix:/tmp/io.podman
```

## SEE ALSO
podman(1), podman-varlink(1)

## HISTORY
January 2020, Originally compiled by Brent Baude<bbaude@redhat.com>
