% podman-container-runlabel(1)

## NAME
podman-container-runlabel - Executes a command as described by a container-image label

## SYNOPSIS
**podman container runlabel** [*options*] *label* *image* [*arg...*]

## DESCRIPTION
**podman container runlabel** reads the specified `label` of the `image` and executes it as command on the host.  If the label does not exist, Podman will exit with an error.  Additional arguments will be appended to the command.

Historically, container images describe the contents (e.g., layers) and how a container runtime (e.g., crun(1) or runc(1)) should execute the container.  For instance, an image may set the environment and the command in its configuration.  However, a container image cannot directly specify how a container engine such as Podman should execute it.  For instance, an image configuration does not include information about log drivers, namespaces or which capabilities it needs to run correctly.

`podman container runlabel` addresses the limitation of container images in a simple yet efficient way.  Podman will read the contents of the label and interpret it as a command that will be executed on the host.  This way an image can describe exactly how it should be executed by Podman.  For instance, a label with the content `/usr/bin/podman run -d --pid=host --privileged \${IMAGE}` instructs the image to be executed in a detached, privileged container that is using the PID namespace of the host.  This lifts the self-description of a container image from "what" to "how".

Please note that the `runlabel` command is intended to be run in trusted environments exclusively.  Using the command on untrusted images is not recommended.

## VARIABLES

The contents of a label may refer to the following variables which will be substituted while processing the label.

**IMAGE**
The name of the image.  When executing `podman container runlabel label fedora` the `IMAGE` variable will be replaced with `fedora`.  Valid formats are `IMAGE`, `$IMAGE`, `${IMAGE}` and `=IMAGE`.

**NAME**
As specified by the `--name` option.  The format is identical to the one of the IMAGE attribute.

**PWD**
Will be replaced with the current working directory.

## OPTIONS
#### **--authfile**=*path*

Path of the containers-auth.json(5) file. Default is ${XDG\_RUNTIME\_DIR}/containers/auth.json, which is set using `podman login`.  If the authorization state is not found there, $HOME/.docker/config.json is checked, which is set using `docker login`.

Note: You can also override the default path of the authentication file by setting the REGISTRY\_AUTH\_FILE environment variable. `export REGISTRY_AUTH_FILE=path`

#### **--display**

Display the label's value of the image having populated its environment variables.  The runlabel command will not execute if --display is specified.

#### **--cert-dir**=*path*

Use certificates at *path* (\*.crt, \*.cert, \*.key) to connect to the registry. (Default: /etc/containers/certs.d)
Please refer to containers-certs.d(5) for details. (This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)

#### **--creds**=*[username[:password]]*

The [username[:password]] to use to authenticate with the registry if required.  If one or both values are not supplied, a command line prompt will appear and the value can be entered.  The password is entered without echo.

#### **--help**, **-h**
Print usage statement

#### **--name**, **-n**=*name*

Use this name for creating content for the container.  If not specified, name defaults to the name of the image.

#### **--quiet**, **-q**

Suppress output information when pulling images

#### **--replace**

If a container exists of the default or given name, as needed it will be stopped, deleted and a new container will be created from this image.

#### **--tls-verify**

Require HTTPS and verify certificates when contacting registries (default: true). If explicitly set to true, then TLS verification will be used. If set to false, then TLS verification will not be used. If not specified, TLS verification will be used unless the target registry is listed as an insecure registry in containers-registries.conf(5).

## EXAMPLES

Execute the `run` label of an image called foobar.
```
$ podman container runlabel run foobar
```

Execute the `install` label of an image called foobar with additional arguments.
```
$ podman container runlabel install foobar apples oranges
```

Display the contents of the `run` label of image foobar.
```
$ podman container runlabel --display run foobar
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[crun(1)](https://github.com/containers/crun/blob/main/crun.1.md)**, **[runc(8)](https://github.com/opencontainers/runc/blob/master/man/runc.8.md)**, **[containers-certs.d(5)](https://github.com/containers/image/blob/main/docs/containers-certs.d.5.md)**, **[containers-auth.json(5)](https://github.com/containers/image/blob/main/docs/containers-auth.json.5.md)**, **[containers-registries.conf(5)](https://github.com/containers/image/blob/main/docs/containers-registries.conf.5.md)**

## HISTORY
August 2021, Refinements by Valentin Rothberg (rothberg at redhat dot com)

September 2018, Originally compiled by Brent Baude (bbaude at redhat dot com)
