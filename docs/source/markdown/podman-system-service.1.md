% podman-system-service 1

## NAME
podman\-system\-service - Run an API service

## SYNOPSIS
**podman system service** [*options*]

## DESCRIPTION
The **podman system service** command creates a listening service that answers API calls for Podman.
The command is available on Linux systems and is usually executed in systemd services.
The command is not available when the Podman command is executed directly on a Windows or macOS
host or in other situations where the Podman command is accessing a remote Podman API service.

The REST API provided by **podman system service** is split into two parts: a compatibility layer offering support for the Docker v1.40 API, and a Podman-native Libpod layer.
Documentation for the latter is available at *https://docs.podman.io/en/latest/_static/api.html*.
Both APIs are versioned, but the server does not reject requests with an unsupported version set.

### Run the command in a systemd service

The command **podman system service** supports systemd socket activation.
When the command is run in a systemd service, the API service can therefore be provided on demand.
If the systemd service is not already running, it will be activated as soon as
a client connects to the listening socket. Systemd then executes the
**podman system service** command.
After some time of inactivity, as defined by the __--time__ option, the command terminates.
Systemd sets the _podman.service_ state as inactive. At this point there is no
**podman system service** process running. No unnecessary compute resources are wasted.
As soon as another client connects, systemd activates the systemd service again.

The systemd unit files that declares the Podman API service for users are

* _/usr/lib/systemd/user/podman.service_
* _/usr/lib/systemd/user/podman.socket_

In the file _podman.socket_ the path of the listening Unix socket is defined by

```
ListenStream=%t/podman/podman.sock
```

The path contains the systemd specifier `%t` which systemd expands to the value of the environment variable
`XDG_RUNTIME_DIR` (see systemd specifiers in the **systemd.unit(5)** man page).

In addition to the systemd user services, there is also a systemd system service _podman.service_.
It runs rootful Podman and is accessed from the Unix socket _/run/podman/podman.sock_. See the systemd unit files

* _/usr/lib/systemd/system/podman.service_
* _/usr/lib/systemd/system/podman.socket_

The **podman system service** command does not support more than one listening socket for the API service.

Note: The default systemd unit files (system and user) change the log-level option to *info* from *error*. This change provides additional information on each API call.

### Run the command directly

To support running an API service without using a systemd service, the command also takes an
optional endpoint argument for the API in URI form.  For example, *unix:///tmp/foobar.sock* or *tcp://localhost:8080*.
If no endpoint is provided, defaults is used.  The default endpoint for a rootful
service is *unix:///run/podman/podman.sock* and rootless is *unix://$XDG_RUNTIME_DIR/podman/podman.sock* (for
example *unix:///run/user/1000/podman/podman.sock*)

### Access the Unix socket from inside a container

To access the API service inside a container:
- mount the socket as a volume
- run the container with `--security-opt label=disable`

Please note that the API grants full access to Podman's capabilities, and allows arbitrary code execution as the user running the API.
We strongly recommend against making the API socket available via the network.
The default configuration (a Unix socket with permissions set to only allow the user running Podman) is the most secure way of running the API.

## OPTIONS

#### **--cors**

CORS headers to inject to the HTTP response. The default value is empty string which disables CORS headers.

#### **--help**, **-h**

Print usage statement.

#### **--time**, **-t**

The time until the session expires in _seconds_. The default is 5
seconds. A value of `0` means no timeout, therefore the session does not expire.

The default timeout can be changed via the `service_timeout=VALUE` field in containers.conf.
See **[containers.conf(5)](https://github.com/containers/common/blob/main/docs/containers.conf.5.md)** for more information.

## EXAMPLES

To start the systemd socket for a rootless service, run as the user:

```
systemctl --user start podman.socket
```

The socket can then be used by for example docker-compose that needs a Docker-compatible API.

```
$ ls $XDG_RUNTIME_DIR/podman/podman.sock
/run/user/1000/podman/podman.sock
$ export DOCKER_HOST=unix://$XDG_RUNTIME_DIR/podman/podman.sock
$ docker-compose up
```

To configure the systemd socket to be automatically started after reboots, run as the user:

```
systemctl --user enable podman.socket
loginctl enable-linger <USER>
```

To start the systemd socket for the rootful service, run:

```
sudo systemctl start podman.socket
```

To configure the socket to be automatically started after reboots, run:

```
sudo systemctl enable podman.socket
```

It is possible to run the API without using systemd socket activation.
In this case the API will not be available on demand because the command will
stay terminated after the inactivity timeout has passed.
Run an API with an inactivity timeout of 5 seconds without using socket activation:

```
podman system service --time 5
```

The default socket was used as no URI argument was provided.

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-system-connection(1)](podman-system-connection.1.md)**, **[containers.conf(5)](https://github.com/containers/common/blob/main/docs/containers.conf.5.md)**

## HISTORY
January 2020, Originally compiled by Brent Baude `<bbaude@redhat.com>`
November 2020, Updated by Jhon Honce (jhonce at redhat dot com)
