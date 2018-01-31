% podman(1) podman-build - Simple tool to build a container image
% Tom Sweeney
# podman-build "7" "December 2017" "podman"

## NAME
podman-build - Build a container image using a Dockerfile.

## SYNOPSIS
**podman** **build** [*options* [...]] [**context**]

## DESCRIPTION
**podman build** Builds an image using instructions from one or more Dockerfiles and a specified
build context directory.  The build context directory can be specified as the
**http** or **https** URL of an archive which will be retrieved and extracted
to a temporary location.  This command passes the parameters entered in by the user to the
**buildah bud** command https://github.com/projectatomic/buildah/blob/master/docs/buildah-bud.md
to do the actual building.

**podman [GLOBAL OPTIONS]**

**podman build [GLOBAL OPTIONS]**

**podman build [OPTIONS] NAME[:TAG|@DIGEST]**

## OPTIONS

**--authfile** *path*

Path of the authentication file. Default is ${XDG_RUNTIME\_DIR}/containers/auth.json, which is set using `podman login`.
If the authorization state is not found there, $HOME/.docker/config.json is checked, which is set using `docker login`.

**--build-arg** *arg=value*

Specifies a build argument and its value, which will be interpolated in
instructions read from the Dockerfiles in the same way that environment
variables are, but which will not be added to environment variable list in the
resulting image's configuration.

**--cert-dir** *path*

Use certificates at *path* (*.crt, *.cert, *.key) to connect to the registry

**--creds** *creds*

The [username[:password]] to use to authenticate with the registry if required.
If one or both values are not supplied, a command line prompt will appear and the
value can be entered.  The password is entered without echo.

**-f, --file** *Dockerfile*

Specifies a Dockerfile which contains instructions for building the image,
either a local file or an **http** or **https** URL.  If more than one
Dockerfile is specified, *FROM* instructions will only be accepted from the
first specified file.

If a build context is not specified, and at least one Dockerfile is a
local file, the directory in which it resides will be used as the build
context.

**--format**

Control the format for the built image's manifest and configuration data.
Recognized formats include *oci* (OCI image-spec v1.0, the default) and
*docker* (version 2, using schema format 2 for the manifest).

**--pull-always**

Pull the image even if a version of the image is already present.

**-q, --quiet**

Suppress output messages which indicate which instruction is being processed,
and of progress when pulling images from a registry, and when writing the
output image.

**--runtime** *path*

The *path* to an alternate OCI-compatible runtime, which will be used to run
commands specified by the **RUN** instruction.

**--runtime-flag** *flag*

Adds global flags for the container runtime.

**--signature-policy** *signature-policy-file*

Path name of a signature policy file to use.  It is not recommended that this
option be used, as the default behavior of using the system-wide default policy
(frequently */etc/containers/policy.json*) is most often preferred.

**-t, --tag** *imageName*

Specifies the name which will be assigned to the resulting image if the build
process completes successfully.

**--tls-verify** *bool-value*

Require HTTPS and verify certificates when talking to container registries (defaults to true)

## EXAMPLES

podman build .

podman build -f Dockerfile.simple .

podman build -f Dockerfile.simple -f Dockerfile.notsosimple

podman build -t imageName .

podman build --tls-verify=true -t imageName -f Dockerfile.simple

podman build --tls-verify=false -t imageName .

podman bud --runtime-flag log-format=json .

podman bud --runtime-flag debug .

podman bud --authfile /tmp/auths/myauths.json --cert-dir ~/auth --tls-verify=true --creds=username:password -t imageName -f Dockerfile.simple

## SEE ALSO
podman(1), buildah(1)

## HISTORY
December 2017, Originally compiled by Tom Sweeney <tsweeney@redhat.com>
