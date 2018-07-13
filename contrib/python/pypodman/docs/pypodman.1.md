% pypodman "1"

## NAME

pypodman - Simple management tool for containers and images

## SYNOPSIS

**pypodman** [*global options*] _command_ [*options*]

## DESCRIPTION

pypodman is a simple client only tool to help with debugging issues when daemons
such as CRI runtime and the kubelet are not responding or failing. pypodman uses
a VarLink API to commicate with a podman service running on either the local or
remote machine.  pypodman uses ssh to create secure tunnels when communicating
with a remote service.

## GLOBAL OPTIONS

**--help, -h**

Print usage statement.

**--version**

Print program version number and exit.

**--config-home**

Directory that will be namespaced with `pypodman` to hold `pypodman.conf`. See FILES below for more details.

**--log-level**

Log events above specified level: DEBUG, INFO, WARNING (default), ERROR, or CRITICAL.

**--run-dir**

Directory that will be namespaced with `pypodman` to hold local socket bindings. The default is ``$XDG_RUNTIME_DIR\`.

**--user**

Authenicating user on remote host.  `pypodman` defaults to the logged in user.

**--host**

Name of remote host.  There is no default, if not given `pypodman` attempts to connect to `--remote-socket-path` on local host.

**--remote-socket-path**

Path on remote host for podman service's `AF_UNIX` socket.  The default is `/run/podman/io.projectatomic.podman`.

**--identity-file**

The optional `ssh` identity file to authenicate when tunnelling to remote host. Default is None and will allow `ssh` to follow it's default methods for resolving the identity and private key using the logged in user.

## COMMANDS

See [podman(1)](podman.1.md)

## FILES

**pypodman/pypodman.conf** (`Any element of XDG_CONFIG_DIRS` and/or `XDG_CONFIG_HOME` and/or **--config-home**)

pypodman.conf is one or more configuration files for running the pypodman command. pypodman.conf is a TOML file with the stanza `[default]`, with a map of option: value.

pypodman follows the XDG (freedesktop.org) conventions for resolving it's configuration. The list below are read from top to bottom with later items overwriting earlier.  Any missing items are ignored.

-   `pypodman/pypodman.conf` from any path element in `XDG_CONFIG_DIRS` or `\etc\xdg`
-   `XDG_CONFIG_HOME` or $HOME/.config + `pypodman/pypodman.conf`
-   From `--config-home` command line option + `pypodman/pypodman.conf`
-   From environment variable, for example: RUN_DIR
-   From command line option, for example: --run-dir

This should provide Operators the ability to setup basic configurations and allow users to customize them.

**XDG_RUNTIME_DIR** (`XDG_RUNTIME_DIR/io.projectatomic.podman`)

Directory where pypodman stores non-essential runtime files and other file objects (such as sockets, named pipes, ...).

## SEE ALSO
`podman(1)`, `libpod(8)`
