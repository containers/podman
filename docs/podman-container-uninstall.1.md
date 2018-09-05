% PODMAN(1) Podman Man Pages
% Dan Walsh
% January 2015
# NAME
podman-container-uninstall - Remove/Uninstall container/container image from system

# SYNOPSIS
**podman container uninstall**
[**--display**]
[**-f**][**--force**]
[**-h**|**--help**]
[**-n**][**--name**[=*NAME*]]
[**--storage**]
IMAGE [ARG...]

# DESCRIPTION
**podman container uninstall** attempts to read the `LABEL UNINSTALL` field in the
container IMAGE, if this field does not exist **podman container uninstall** will just
uninstall the image.

The image won't be removed if there are containers using it and `--force` is not used.

If the container image has a LABEL UNINSTALL instruction like the following:

`LABEL UNINSTALL /usr/bin/podman run -t -i --rm \${OPT1} --privileged -v /:/host --net=host --ipc=host --pid=host -e HOST=/host -e NAME=${NAME} -e IMAGE=${IMAGE} -e CONFDIR=\/etc/${NAME} -e LOGDIR=/var/log/\${NAME} -e DATADIR=/var/lib/\${NAME} ${IMAGE} \${OPT2} /bin/uninstall.sh \${OPT3}`

`podman container uninstall` will set the following environment variables for use in the command:

**NAME**
  The name specified via the command.  NAME will be replaced with IMAGE if it is not specified.

**IMAGE**
  The name and image specified via the command.

**OPT1, OPT2, OPT3**
  Additional options which can be specified via the command.

**SUDO_UID**
  The `SUDO_UID` environment variable.  This is useful with the docker `-u` option for user space tools.  If the environment variable is not available, the value of `/proc/self/loginuid` is used.

**SUDO_GID**
  The `SUDO_GID` environment variable.  This is useful with the docker `-u` option for user space tools.  If the environment variable is not available, the default GID of the value for `SUDO_UID` is used.  If this value is not available, the value of `/proc/self/loginuid` is used.

Any additional arguments will be appended to the command.

# OPTIONS:
**--display**
Display the image's uninstall options and environment variables
populated into the uninstall command.
The uninstall command will not execute if --display is specified.
If --display is not specified the uninstall command will execute.

**-f** **--force**
  Remove all containers based on this image before removing the image.

**-h** **--help**
  Print usage statement

**-n** **--name**=""
   If name is specified `podman container uninstall` will uninstall the named container from the system, otherwise it will uninstall the container images.

**--storage**
The --storage option will direct podman container where it should look for the image
prior to uninstalling. Valid options are `docker` and `ostree`.

## SEE ALSO
podman(1), podman-container-install(5)

# HISTORY
September 2018, Originally compiled by Daniel Walsh (dwalsh at redhat dot com)
