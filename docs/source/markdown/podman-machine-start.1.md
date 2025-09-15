% podman-machine-start 1

## NAME
podman\-machine\-start - Start a virtual machine

## SYNOPSIS
**podman machine start** [*name*]

## DESCRIPTION

Starts a virtual machine for Podman.

Rootless only.

Podman on MacOS and Windows requires a virtual machine. This is because containers are Linux -
containers do not run on any other OS because containers' core functionality are
tied to the Linux kernel. Podman machine must be used to manage MacOS and Windows machines,
but can be optionally used on Linux.

The default machine name is `podman-machine-default`. If a machine name is not specified as an argument,
then `podman-machine-default` will be started.

Only one Podman managed VM can be active at a time. If a VM is already running,
`podman machine start` returns an error.

**podman machine start** starts a Linux virtual machine where containers are run.

## OPTIONS

#### **--help**

Print usage statement.

#### **--no-info**

Suppress informational tips.

#### **--quiet**, **-q**

Suppress machine starting status output.

## EXAMPLES

Start the specified podman machine.
```
$ podman machine start myvm
Starting machine "myvm"
This machine is currently configured in rootless mode. If your containers
require root permissions (e.g. ports < 1024), or if you run into compatibility
issues with non-podman clients, you can switch using the following command:

	podman machine set --rootful myvm

API forwarding listening on: /var/folders/my/k1kpd8ds7p938mchyb1r_4mw0000gn/T/podman/myvm-api.sock

The system helper service is not installed; the default Docker API socket
address can't be used by podman. If you would like to install it, run the following commands:

        sudo /opt/homebrew/Cellar/podman/5.2.4/bin/podman-mac-helper install
        podman machine stop myvm; podman machine start myvm

You can still connect Docker API clients by setting DOCKER_HOST using the
following command in your terminal session:

        export DOCKER_HOST='unix:///var/folders/my/k1kpd8ds7p938mchyb1r_4mw0000gn/T/podman/myvm-api.sock'

Machine "myvm" started successfully
```

Start with --no-info:
```
$ podman machine start --no-info myvm
Starting machine "myvm"
Machine "myvm" started successfully
```

Start with --quiet:
```
$ podman machine start --quiet myvm
Machine "myvm" started successfully
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine(1)](podman-machine.1.md)**

## HISTORY
March 2021, Originally compiled by Ashley Cui <acui@redhat.com>
