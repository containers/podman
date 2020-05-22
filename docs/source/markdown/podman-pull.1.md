% podman-pull(1)

## NAME
podman\-pull - Pull an image from a registry

## SYNOPSIS
**podman pull** [*options*] *source*

**podman image pull** [*options*] *source*

**podman pull** [*options*] [*transport*]*name*[:*tag*|@*digest*]

**podman image pull** [*options*] [*transport*]*name*[:*tag*|@*digest*]

## DESCRIPTION
Copies an image from a registry onto the local machine. **podman pull** pulls an
image from Docker Hub if a registry is not specified in the command line argument.
If an image tag is not specified, **podman pull** defaults to the image with the
**latest** tag (if it exists) and pulls it. After the image is pulled, podman will
print the full image ID.  **podman pull** can also pull an image
using its digest **podman pull** *image*@*digest*. **podman pull** can be used to pull
images from archives and local storage using different transports.

## Image storage
Images are stored in local image storage.

## SOURCE

 The SOURCE is the location from which the container images are pulled.
 The Image "SOURCE" uses a "transport":"details" format.

 Multiple transports are supported:

  **dir:**_path_
  An existing local directory _path_ storing the manifest, layer tarballs and signatures as individual files. This is a non-standardized format, primarily useful for debugging or noninvasive container inspection.

  **docker://**_docker-reference_
  An image in a registry implementing the "Docker Registry HTTP API V2". By default, uses the authorization state in `$XDG_RUNTIME_DIR/containers/auth.json`, which is set using `(podman login)`. If the authorization state is not found there, `$HOME/.docker/config.json` is checked, which is set using `(docker login)`.

  **docker-archive:**_path_[**:**_docker-reference_]
  An image is stored in the `docker save` formatted file.  _docker-reference_ is only used when creating such a file, and it must not contain a digest.

  **docker-daemon:**_docker-reference_
  An image _docker-reference_ stored in the docker daemon internal storage.  _docker-reference_ must contain either a tag or a digest.  Alternatively, when reading images, the format can also be docker-daemon:algo:digest (an image ID).

  **oci-archive:**_path_**:**_tag_
  An image _tag_ in a directory compliant with "Open Container Image Layout Specification" at _path_.

## OPTIONS

**--all-tags**, **a**

All tagged images in the repository will be pulled.

Note: When using the all-tags flag, Podman will not iterate over the search registries in the containers-registries.conf(5) but will always use docker.io for unqualified image names.

**--authfile**=*path*

Path of the authentication file. Default is ${XDG\_RUNTIME\_DIR}/containers/auth.json, which is set using `podman login`.
If the authorization state is not found there, $HOME/.docker/config.json is checked, which is set using `docker login`. (Not available for remote commands)

Note: You can also override the default path of the authentication file by setting the REGISTRY\_AUTH\_FILE
environment variable. `export REGISTRY_AUTH_FILE=path`

**--cert-dir**=*path*

Use certificates at *path* (\*.crt, \*.cert, \*.key) to connect to the registry.
Default certificates directory is _/etc/containers/certs.d_. (Not available for remote commands)

**--creds**=*[username[:password]]*

The [username[:password]] to use to authenticate with the registry if required.
If one or both values are not supplied, a command line prompt will appear and the
value can be entered.  The password is entered without echo.

**--override-os**=*OS*
Use OS instead of the running OS for choosing images

**--override-arch**=*ARCH*

Override the machine's default architecture of the image to be pulled. For example, `arm`.

**--quiet**, **-q**

Suppress output information when pulling images

**--tls-verify**=*true|false*

Require HTTPS and verify certificates when contacting registries (default: true). If explicitly set to true,
then TLS verification will be used. If set to false, then TLS verification will not be used. If not specified,
TLS verification will be used unless the target registry is listed as an insecure registry in registries.conf. (Not available for remote commands)

**--help**, **-h**

Print usage statement

## EXAMPLES

```
$ podman pull alpine:latest
Trying to pull registry.access.redhat.com/alpine:latest... Failed
Trying to pull registry.fedoraproject.org/alpine:latest... Failed
Trying to pull docker.io/library/alpine:latest...Getting image source signatures
Copying blob sha256:88286f41530e93dffd4b964e1db22ce4939fffa4a4c665dab8591fbab03d4926
 1.90 MB / 1.90 MB [========================================================] 0s
Copying config sha256:76da55c8019d7a47c347c0dceb7a6591144d232a7dd616242a367b8bed18ecbc
 1.48 KB / 1.48 KB [========================================================] 0s
Writing manifest to image destination
Storing signatures
04660052281190168dbb2362eb15bf7067a8dc642d2498055e0e72efa961a4b6
```

```
$ podman pull --authfile temp-auths/myauths.json docker://docker.io/umohnani/finaltest
Trying to pull docker.io/umohnani/finaltest:latest...Getting image source signatures
Copying blob sha256:6d987f6f42797d81a318c40d442369ba3dc124883a0964d40b0c8f4f7561d913
 1.90 MB / 1.90 MB [========================================================] 0s
Copying config sha256:ad4686094d8f0186ec8249fc4917b71faa2c1030d7b5a025c29f26e19d95c156
 1.41 KB / 1.41 KB [========================================================] 0s
Writing manifest to image destination
Storing signatures
03290064078cb797f3e0a530e78c20c13dd22a3dd3adf84a5da2127b48df0438
```

```
$ podman pull --creds testuser:testpassword docker.io/umohnani/finaltest
Trying to pull docker.io/umohnani/finaltest:latest...Getting image source signatures
Copying blob sha256:6d987f6f42797d81a318c40d442369ba3dc124883a0964d40b0c8f4f7561d913
 1.90 MB / 1.90 MB [========================================================] 0s
Copying config sha256:ad4686094d8f0186ec8249fc4917b71faa2c1030d7b5a025c29f26e19d95c156
 1.41 KB / 1.41 KB [========================================================] 0s
Writing manifest to image destination
Storing signatures
03290064078cb797f3e0a530e78c20c13dd22a3dd3adf84a5da2127b48df0438
```

```
$ podman pull --tls-verify=false --cert-dir image/certs docker.io/umohnani/finaltest
Trying to pull docker.io/umohnani/finaltest:latest...Getting image source signatures
Copying blob sha256:6d987f6f42797d81a318c40d442369ba3dc124883a0964d40b0c8f4f7561d913
 1.90 MB / 1.90 MB [========================================================] 0s
Copying config sha256:ad4686094d8f0186ec8249fc4917b71faa2c1030d7b5a025c29f26e19d95c156
 1.41 KB / 1.41 KB [========================================================] 0s
Writing manifest to image destination
Storing signatures
03290064078cb797f3e0a530e78c20c13dd22a3dd3adf84a5da2127b48df0438
```

```
$ podman pull --override-arch=arm arm32v7/debian:stretch
Trying to pull docker.io/arm32v7/debian:stretch...
Getting image source signatures
Copying blob b531ae4a3925 done
Copying config 3cba58dad5 done
Writing manifest to image destination
Storing signatures
3cba58dad5d9b35e755b48b634acb3fdd185ab1c996ac11510cc72c17780e13c
```

## FILES

**registries.conf** (`/etc/containers/registries.conf`)

	registries.conf is the configuration file which specifies which container registries should be consulted when completing image names which do not include a registry or domain portion.

NOTE: Use the environment variable `TMPDIR` to change the temporary storage location of downloaded container images. Podman defaults to use `/var/tmp`.

## SEE ALSO
podman(1), podman-push(1), podman-login(1), containers-registries.conf(5)

## HISTORY
July 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
