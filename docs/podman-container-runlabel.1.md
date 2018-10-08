% PODMAN(1) Podman Man Pages
% Brent Baude
% September 2018
# NAME
podman-container-runlabel - Execute Image Label Method

# SYNOPSIS
**podman container runlabel**
[**-h**|**--help**]
[**--display**]
[**-n**][**--name**[=*NAME*]]
[**-p**][[**--pull**]]
[**--rootfs**=*ROOTFS*]
[**--set**=*NAME*=*VALUE*]
[**--storage**]
LABEL IMAGE [ARG...]

# DESCRIPTION
**podman container runlabel** reads the provided `LABEL` field in the container
IMAGE and executes the provided value for the label as a command. If this field does not
exist, `podman container runlabel` will just exit.

If the container image has a LABEL INSTALL instruction like the following:

`LABEL INSTALL /usr/bin/podman run -t -i --rm \${OPT1} --privileged -v /:/host --net=host --ipc=host --pid=host -e HOST=/host -e NAME=\${NAME} -e IMAGE=\${IMAGE} -e CONFDIR=\/etc/${NAME} -e LOGDIR=/var/log/\${NAME} -e DATADIR=/var/lib/\${NAME} \${IMAGE} \${OPT2} /bin/install.sh \${OPT3}`

`podman container runlabel` will set the following environment variables for use in the command:

Note: Podman will always ensure that `podman` is the first argument of the command being executed.

**LABEL**
The label name specified via the command.

**IMAGE**
Image name specified via the command.

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

Display the label's value of the image having populated its environment variables.
The runlabel command will not execute if --display is specified.

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
 Use this name for creating content for the container. NAME will default to the IMAGENAME if it is not specified.

**p** **--pull**
 Pull the image if it cannot be found in local storage.

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

## Examples ##

Execute the run label of an image called foobar.
```
$ sudo podman container runlabel run foobar
```

Execute the install label of an image called foobar with additional arguments.
```
$ sudo podman container runlabel install foobar apples oranges
```

Display the command that would be executed by runlabel.
```
$ sudo podman container runlabel --display run foobar
```

## SEE ALSO
podman(1)

# HISTORY
September 2018, Originally compiled by Brent Baude (bbaude at redhat dot com)
