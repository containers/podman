% podman-pull(1)

## NAME
podman\-pull - Pull an image from a registry

## SYNOPSIS
**podman pull** [*options*] *source*

**podman image pull** [*options*] *source*

**podman pull** [*options*] [*transport*]*name*[:*tag*|@*digest*]

**podman image pull** [*options*] [*transport*]*name*[:*tag*|@*digest*]

## DESCRIPTION
Copies an image from a registry onto the local machine. The **podman pull** command pulls an
image.  If the image reference in the command line argument does not contain a registry, it is referred to as a`short-name` reference. If the image is a 'short-name' reference, Podman will prompt the user for the specific container registry to pull the image from, if an alias for the short-name has not been specified in the short-name-aliases.conf.  If an image tag is not specified, **podman pull** defaults to the image with the **latest** tag (if it exists) and pulls it. After the image is pulled, podman will print the full image ID.  **podman pull** can also pull an image using its digest **podman pull** *image*@*digest*. **podman pull** can be used to pull images from archives and local storage using different transports.

## Image storage
Images are stored in local image storage.

## SOURCE

 The SOURCE is the location from which the container images are pulled.
 The Image "SOURCE" uses a "transport":"details" format.  Only the `docker` (container registry)
 transport is allowed for remote access.

 Multiple transports are supported:

  **dir:**_path_
  An existing local directory _path_ storing the manifest, layer tarballs and signatures as individual files. This
  is a non-standardized format, primarily useful for debugging or noninvasive container inspection.

    $ podman pull dir:/tmp/myimage

  **docker://**_docker-reference_ (Default)
  An image reference stored in a remote container image registry. The reference can include a path to a
  specific registry; if it does not, the registries listed in registries.conf will be queried to find a matching
  image. By default, credentials from podman login (stored at $XDG_RUNTIME_DIR/containers/auth.json by default)
  will  be used to authenticate; if these cannot be found, we will fall back to using credentials in
  $HOME/.docker/config.json.

    $ podman pull quay.io/username/myimage

  **docker-archive:**_path_[**:**_docker-reference_]
  An image is stored in the `docker save` formatted file.  _docker-reference_ is only used when creating such a
  file, and it must not contain a digest.

    $ podman pull docker-archive:/tmp/myimage

  **docker-daemon:**_docker-reference_
  An image in _docker-reference_ format stored in the docker daemon internal storage. The _docker-reference_ can also be an image ID (docker-daemon:algo:digest).

    $ sudo podman pull docker-daemon:docker.io/library/myimage:33

  **oci-archive:**_path_**:**_tag_
  An image _tag_ in a directory compliant with "Open Container Image Layout Specification" at _path_.

    $ podman pull oci-archive:/tmp/myimage

## OPTIONS

#### **--all-tags**, **a**

All tagged images in the repository will be pulled.

Note: When using the all-tags flag, Podman will not iterate over the search registries in the containers-registries.conf(5) but will always use docker.io for unqualified image names.

#### **--arch**=*ARCH*
Override the architecture, defaults to hosts, of the image to be pulled. For example, `arm`.

#### **--authfile**=*path*

Path of the authentication file. Default is ${XDG\_RUNTIME\_DIR}/containers/auth.json, which is set using `podman login`.
If the authorization state is not found there, $HOME/.docker/config.json is checked, which is set using `docker login`.

Note: You can also override the default path of the authentication file by setting the REGISTRY\_AUTH\_FILE
environment variable. `export REGISTRY_AUTH_FILE=path`

#### **--cert-dir**=*path*

Use certificates at *path* (\*.crt, \*.cert, \*.key) to connect to the registry.
Please refer to containers-certs.d(5) for details. (This option is not available with the remote Podman client)

#### **--creds**=*[username[:password]]*

The [username[:password]] to use to authenticate with the registry if required.
If one or both values are not supplied, a command line prompt will appear and the
value can be entered.  The password is entered without echo.

#### **--disable-content-trust**

This is a Docker specific option to disable image verification to a Docker
registry and is not supported by Podman.  This flag is a NOOP and provided
solely for scripting compatibility.

#### **--help**, **-h**

Print usage statement

#### **--os**=*OS*
Override the OS, defaults to hosts, of the image to be pulled. For example, `windows`.

#### **--platform**=*OS/ARCH*

Specify the platform for selecting the image.  (Conflicts with --arch and --os)
The `--platform` option can be used to override the current architecture and operating system.

#### **--quiet**, **-q**

Suppress output information when pulling images

#### **--tls-verify**=*true|false*

Require HTTPS and verify certificates when contacting registries (default: true). If explicitly set to true,
then TLS verification will be used. If set to false, then TLS verification will not be used. If not specified,
TLS verification will be used unless the target registry is listed as an insecure registry in registries.conf.

#### **--variant**=*VARIANT*

Use _VARIANT_ instead of the default architecture variant of the container image.  Some images can use multiple variants of the arm architectures, such as arm/v5 and arm/v7.

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
$ podman pull alpine@sha256:d7342993700f8cd7aba8496c2d0e57be0666e80b4c441925fc6f9361fa81d10e
Trying to pull docker.io/library/alpine@sha256:d7342993700f8cd7aba8496c2d0e57be0666e80b4c441925fc6f9361fa81d10e...
Getting image source signatures
Copying blob 188c0c94c7c5 done
Copying config d6e46aa247 done
Writing manifest to image destination
Storing signatures
d6e46aa2470df1d32034c6707c8041158b652f38d2a9ae3d7ad7e7532d22ebe0
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
$ podman pull --arch=arm arm32v7/debian:stretch
Trying to pull docker.io/arm32v7/debian:stretch...
Getting image source signatures
Copying blob b531ae4a3925 done
Copying config 3cba58dad5 done
Writing manifest to image destination
Storing signatures
3cba58dad5d9b35e755b48b634acb3fdd185ab1c996ac11510cc72c17780e13c
```

## FILES

**short-name-aliases.conf** (`/var/cache/containers/short-name-aliases.conf`, `$HOME/.cache/containers/short-name-aliases.conf`)

When users specify images that do not include the container registry where the
image is stored, this is called a short name. The use of unqualified-search registries entails an ambiguity as it is unclear from which registry a given image, referenced by a short name, may be pulled from.

Using short names is subject to the risk of hitting squatted registry namespaces. If the unqualified-search registries are set to ["public-registry.com",  "my-private-registry.com"] an attacker may take over a namespace of `public-registry.com` such that an image may be pulled from `public-registry.com` instead of the intended source `my-private-registry.com`.

While it is highly recommended to always use fully-qualified image references, existing deployments using short names may not be easily changed. To circumvent the aforementioned ambiguity, so called short-name aliases can be configured that point to a fully-qualified image reference. Distributions often ship a default shortnames.conf expansion file in /etc/containers/registries.conf.d/ directory. Administrators can use this directory to add their own local short-name expansion files.

When pulling an image, if the user does not specify the complete registry, container engines attempt to expand the short-name into a full name. If the command is executed with a tty, the user will be prompted to select a registry from the
default list unqualified registries defined in registries.conf. The user's selection is then stored in a cache file to be used in all future short-name expansions. Rootfull short-names are stored in /var/cache/containers/short-name-aliases.conf. Rootless short-names are stored in the $HOME/.cache/containers/short-name-aliases.conf file.

For more information on short-names, see `containers-registries.conf(5)`

**registries.conf** (`/etc/containers/registries.conf`)

registries.conf is the configuration file which specifies which container registries should be consulted when completing image names which do not include a registry or domain portion.

NOTE: Use the environment variable `TMPDIR` to change the temporary storage location of downloaded container images. Podman defaults to use `/var/tmp`.

## SEE ALSO
podman(1), podman-push(1), podman-login(1), containers-certs.d(5), containers-registries.conf(5)

## HISTORY
July 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
