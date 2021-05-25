% podman-container-runlabel(1)

## NAME
podman-container-runlabel - Executes a command as described by a container image label

## SYNOPSIS
**podman container runlabel** [*options*] *label* *image* [*arg...*]

## DESCRIPTION
**podman container runlabel** reads the provided `LABEL` field in the container
IMAGE and executes the provided value for the label as a command. If this field does not
exist, `podman container runlabel` will just exit.

If the container image has a LABEL INSTALL instruction like the following:

`LABEL INSTALL /usr/bin/podman run -t -i --rm \${OPT1} --privileged -v /:/host --net=host --ipc=host --pid=host -e HOST=/host -e NAME=\${NAME} -e IMAGE=\${IMAGE} -e CONFDIR=/etc/\${NAME} -e LOGDIR=/var/log/\${NAME} -e DATADIR=/var/lib/\${NAME} \${IMAGE} \${OPT2} /bin/install.sh \${OPT3}`

`podman container runlabel` will set the following environment variables for use in the command:

If the container image does not have the desired label, an error message will be displayed along with a non-zero
return code.  If the image is not found in local storage, Podman will attempt to pull it first.

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

## OPTIONS
#### **--authfile**=*path*

Path of the authentication file. Default is ${XDG\_RUNTIME\_DIR}/containers/auth.json, which is set using `podman login`.
If the authorization state is not found there, $HOME/.docker/config.json is checked, which is set using `docker login`.

Note: You can also override the default path of the authentication file by setting the REGISTRY\_AUTH\_FILE
environment variable. `export REGISTRY_AUTH_FILE=path`

#### **--display**

Display the label's value of the image having populated its environment variables.
The runlabel command will not execute if --display is specified.

#### **--cert-dir**=*path*

Use certificates at *path* (\*.crt, \*.cert, \*.key) to connect to the registry.
Please refer to containers-certs.d(5) for details. (This option is not available with the remote Podman client)

#### **--creds**=*[username[:password]]*

The [username[:password]] to use to authenticate with the registry if required.
If one or both values are not supplied, a command line prompt will appear and the
value can be entered.  The password is entered without echo.

#### **--help**, **-h**
Print usage statement

#### **--name**, **-n**=*name*

Use this name for creating content for the container. NAME will default to the IMAGENAME if it is not specified.

#### **--quiet**, **-q**

Suppress output information when pulling images

#### **--replace**

If a container exists of the default or given name, as needed it will be stopped, deleted and a new container will be
created from this image.

#### **--tls-verify**

Require HTTPS and verify certificates when contacting registries (default: true). If explicitly set to true,
then TLS verification will be used. If set to false, then TLS verification will not be used. If not specified,
TLS verification will be used unless the target registry is listed as an insecure registry in registries.conf.

## EXAMPLES

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
podman(1), containers-certs.d(5)

## HISTORY
September 2018, Originally compiled by Brent Baude (bbaude at redhat dot com)
