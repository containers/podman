% PODMAN(1) Podman Man Pages
% Dan Walsh
% January 2015
# NAME
podman-container-install - Execute Image Install Method

# SYNOPSIS
**podman container install**
[**-h**|**--help**]
[**--display**]
[**-n**][**--name**[=*NAME*]]
[**--rootfs**=*ROOTFS*]
[**--set**=*NAME*=*VALUE*]
[**--storage**]
[**--system-package=auto|build|yes|no**]
[**--system**]
IMAGE [ARG...]

# DESCRIPTION
**podman container install** attempts to read the `LABEL INSTALL` field in the container
IMAGE, if this field does not exist, `podman container install` will install the IMAGE.

If the container image has a LABEL INSTALL instruction like the following:

`LABEL INSTALL /usr/bin/podman run -t -i --rm \${OPT1} --privileged -v /:/host --net=host --ipc=host --pid=host -e HOST=/host -e NAME=\${NAME} -e IMAGE=\${IMAGE} -e CONFDIR=\/etc/${NAME} -e LOGDIR=/var/log/\${NAME} -e DATADIR=/var/lib/\${NAME} \${IMAGE} \${OPT2} /bin/install.sh \${OPT3}`

`podman container install` will set the following environment variables for use in the command:

Note: If the command to execute in the container is `/usr/bin/docker`, podman container install will execute /usr/bin/podman instead.

**NAME**
The name specified via the command.  NAME will be replaced with IMAGE if it is not specified.

**IMAGE**
The name and image specified via the command.

**OPT1, OPT2, OPT3**
Additional options which can be specified via the command.

**SUDO_UID**
The `SUDO_UID` environment variable.  This is useful with the podman
`-u` option for user space tools.  If the environment variable is
not available, the value of `/proc/self/loginuid` is used.

**SUDO_GID**
The `SUDO_GID` environment variable.  This is useful with the podman
`-u` option for user space tools.  If the environment variable is
not available, the default GID of the value for `SUDO_UID` is used.
If this value is not available, the value of `/proc/self/loginuid`
is used.

Any additional arguments will be appended to the command.

# OPTIONS:
**-h** **--help**
Print usage statement

**--display**
Display the image's install options and environment variables
populated into the install command.
The install command will not execute if --display is specified.
If --display is not specified the install command will execute.

**-n** **--name**=""
 Use this name for creating installed content for the container.
 NAME will default to the IMAGENAME if it is not specified.

## SEE ALSO
podman(1), podman-container-uninstall(5)

# HISTORY
September 2018, Originally compiled by Daniel Walsh (dwalsh at redhat dot com)
