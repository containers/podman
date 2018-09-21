% PODMAN(1) Podman Man Pages
% Dan Walsh
% September 2018
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
IMAGE, if this field does not exist, `podman container install` will just pull the IMAGE.

If the container image has a LABEL INSTALL instruction like the following:

`LABEL INSTALL /usr/bin/podman run -t -i --rm \${OPT1} --privileged -v /:/host --net=host --ipc=host --pid=host -e HOST=/host -e NAME=\${NAME} -e IMAGE=\${IMAGE} -e CONFDIR=\/etc/${NAME} -e LOGDIR=/var/log/\${NAME} -e DATADIR=/var/lib/\${NAME} \${IMAGE} \${OPT2} /bin/install.sh \${OPT3}`

`podman container install` will set the following environment variables for use in the command:

Note: Podman will always ensure that `podman` is the first argument of the command being executed.

**NAME**
The name specified via the command.  NAME will be replaced with IMAGE if it is not specified.

**IMAGE**
Image name specified via the command.

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
**--authfile**

Path of the authentication file. Default is ${XDG_RUNTIME\_DIR}/containers/auth.json, which is set using `podman login`.
If the authorization state is not found there, $HOME/.docker/config.json is checked, which is set using `docker login`.

**--display**
Display the image's install options and environment variables
populated into the install command.
The install command will not execute if --display is specified.
If --display is not specified the install command will execute.

**--cert-dir** *path*

Use certificates at *path* (\*.crt, \*.cert, \*.key) to connect to the registry.
Default certificates directory is _/etc/containers/certs.d_.

**--creds**

The [username[:password]] to use to authenticate with the registry if required.
If one or both values are not supplied, a command line prompt will appear and the
value can be entered.  The password is entered without echo.

**-h** **--help**
Print usage statement

**-n** **--name**=""
 Use this name for creating installed content for the container.
 NAME will default to the IMAGENAME if it is not specified.

**--quiet, -q**

Suppress output information when pulling images

**--signature-policy="PATHNAME"**

Pathname of a signature policy file to use.  It is not recommended that this
option be used, as the default behavior of using the system-wide default policy
(frequently */etc/containers/policy.json*) is most often preferred

**--tls-verify**

Require HTTPS and verify certificates when contacting registries (default: true). If explicitly set to true,
then tls verification will be used, If set to false then tls verification will not be used. If not specified
tls verification will be used unless the target registry is listed as an insecure registry in registries.conf

## SEE ALSO
podman(1), podman-container-uninstall(5)

# HISTORY
September 2018, Originally compiled by Daniel Walsh (dwalsh at redhat dot com)
