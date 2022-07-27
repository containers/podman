% podman-manifest-add(1)

## NAME
podman\-manifest\-add - Add an image to a manifest list or image index

## SYNOPSIS
**podman manifest add** [*options*] *listnameorindexname* [*transport*]:*imagename*

## DESCRIPTION

Adds the specified image to the specified manifest list or image index.

## RETURN VALUE
The list image's ID.

## OPTIONS

#### **--all**

If the image which should be added to the list or index is itself a list or
index, add all of the contents to the local list.  By default, only one image
from such a list or index will be added to the list or index.  Combining
*--all* with any of the other options described below is NOT recommended.

#### **--annotation**=*annotation=value*

Set an annotation on the entry for the newly-added image.

#### **--arch**

Override the architecture which the list or index records as a requirement for
the image.  If *imageName* refers to a manifest list or image index, the
architecture information will be retrieved from it.  Otherwise, it will be
retrieved from the image's configuration information.

#### **--authfile**=*path*

Path of the authentication file. Default is ${XDG\_RUNTIME\_DIR}/containers/auth.json, which is set using `podman login`.
If the authorization state is not found there, $HOME/.docker/config.json is checked, which is set using `docker login`.

Note: You can also override the default path of the authentication file by setting the REGISTRY\_AUTH\_FILE
environment variable. `export REGISTRY_AUTH_FILE=path`

#### **--cert-dir**=*path*

Use certificates at *path* (\*.crt, \*.cert, \*.key) to connect to the registry. (Default: /etc/containers/certs.d)
Please refer to containers-certs.d(5) for details. (This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)

#### **--creds**=*creds*

The [username[:password]] to use to authenticate with the registry if required.
If one or both values are not supplied, a command line prompt will appear and the
value can be entered.  The password is entered without echo.

#### **--features**

Specify the features list which the list or index records as requirements for
the image.  This option is rarely used.

#### **--os**

Override the OS which the list or index records as a requirement for the image.
If *imagename* refers to a manifest list or image index, the OS information
will be retrieved from it.  Otherwise, it will be retrieved from the image's
configuration information.

#### **--os-version**

Specify the OS version which the list or index records as a requirement for the
image.  This option is rarely used.

#### **--tls-verify**

Require HTTPS and verify certificates when talking to container registries (defaults to true).

#### **--variant**

Specify the variant which the list or index records for the image.  This option
is typically used to distinguish between multiple entries which share the same
architecture value, but which expect different versions of its instruction set.

## Transport

 Multiple transports are supported:

  **docker://**_docker-reference_ _(default)_
  An image in a registry implementing the "Docker Registry HTTP API V2". By default, uses the authorization state in `$XDG_RUNTIME_DIR/containers/auth.json`, which is set using `(podman login)`. If the authorization state is not found there, `$HOME/.docker/config.json` is checked, which is set using `(docker login)`.

    $ podman manifest add mylist:v1.11 docker://quay.io/username/myimage

  **containers-storage:**_oci-reference_
  An image in _oci-reference_ format stored in the local container storage. _oci-reference_ must contain a tag.

    $ podman manifest add mylist:v1.11 containers-storage:quay.io/username/myimage

  **dir:**_path_
  An existing local directory _path_ storing the manifest, layer tarballs, and signatures as individual files. This
  is a non-standardized format, primarily useful for debugging or noninvasive container inspection.

    $ podman manifest add dir:/tmp/myimage

  **docker-archive:**_path_[**:**_docker-reference_]
  An image is stored in the `docker save` formatted file.  _docker-reference_ is only used when creating such a
  file, and it must not contain a digest.

    $ podman manifest add docker-archive:/tmp/myimage

  **docker-daemon:**_docker-reference_
  An image in _docker-reference_ format stored in the docker daemon internal storage. The _docker-reference_ can also be an image ID (docker-daemon:algo:digest).

    $ sudo podman manifest add docker-daemon:docker.io/library/myimage:33

  **oci-archive:**_path_**:**_tag_
  An image _tag_ in a directory compliant with "Open Container Image Layout Specification" at _path_.

    $ podman manifest add oci-archive:/tmp/myimage

## EXAMPLE

```
podman manifest add mylist:v1.11 docker://fedora
71c201d10fffdcac52968a000d85a0a016ca1c7d5473948000d3131c1773d965
```

```
podman manifest add --all mylist:v1.11 docker://fedora
71c201d10fffdcac52968a000d85a0a016ca1c7d5473948000d3131c1773d965
```

```
podman manifest add --arch arm64 --variant v8 mylist:v1.11 docker://71c201d10fffdcac52968a000d85a0a016ca1c7d5473948000d3131c1773d965
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-manifest(1)](podman-manifest.1.md)**
