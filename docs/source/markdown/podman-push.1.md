% podman-push(1)

## NAME
podman\-push - Push an image, manifest list or image index from local storage to elsewhere

## SYNOPSIS
**podman push** [*options*] *image* [*destination*]

**podman image push** [*options*] *image* [*destination*]

## DESCRIPTION
Pushes an image, manifest list or image index from local storage to a specified
destination. Push is mainly used to push images to registries, however
**podman push** can be used to save images to tarballs and directories using the
following transports:
**dir:**, **docker-archive:**, **docker-daemon:** and **oci-archive:**.

## Image storage
Images are pushed from those stored in local image storage.

## DESTINATION

 The DESTINATION is a location to store container images
 The Image "DESTINATION" uses a "transport":"details" format.
 If a transport is not given, podman push will attempt to push
 to a registry.

 Multiple transports are supported:

  **dir:**_path_
  An existing local directory _path_ storing the manifest, layer tarballs and signatures as individual files. This is a non-standardized format, primarily useful for debugging or noninvasive container inspection.

    $ podman push myimage dir:/tmp/myimage

  **docker://**_docker-reference_
  An image in a registry implementing the "Docker Registry HTTP API V2". By default, uses the authorization state in `$XDG_RUNTIME_DIR/containers/auth.json`, which is set using `(podman login)`. If the authorization state is not found there, `$HOME/.docker/config.json` is checked, which is set using `(docker login)`.

    $ podman push myimage quay.io/username/myimage

  **docker-archive:**_path_[**:**_docker-reference_]
  An image is stored in the `docker save` formatted file.  _docker-reference_ is only used when creating such a file, and it must not contain a digest.

    $ podman push myimage docker-archive:/tmp/myimage

  **docker-daemon:**_docker-reference_
  An image in _docker-reference_ format stored in the docker daemon internal storage. _docker-reference_ must contain a tag.

    $ sudo podman push myimage docker-daemon:docker.io/library/myimage:33

  **oci-archive:**_path_**:**_tag_
  An image _tag_ in a directory compliant with "Open Container Image Layout Specification" at _path_.

    $ podman push myimage oci-archive:/tmp/myimage

## OPTIONS

#### **--authfile**=*path*

Path of the authentication file. Default is ${XDG\_RUNTIME\_DIR}/containers/auth.json, which is set using `podman login`.
If the authorization state is not found there, $HOME/.docker/config.json is checked, which is set using `docker login`.

Note: You can also override the default path of the authentication file by setting the REGISTRY\_AUTH\_FILE
environment variable. `export REGISTRY_AUTH_FILE=path`

#### **--creds**=*[username[:password]]*

The [username[:password]] to use to authenticate with the registry if required.
If one or both values are not supplied, a command line prompt will appear and the
value can be entered.  The password is entered without echo.

#### **--cert-dir**=*path*

Use certificates at *path* (\*.crt, \*.cert, \*.key) to connect to the registry.
Please refer to containers-certs.d(5) for details. (This option is not available with the remote Podman client)

#### **--compress**

Compress tarball image layers when pushing to a directory using the 'dir' transport. (default is same compression type, compressed or uncompressed, as source)
Note: This flag can only be set when using the **dir** transport

#### **--digestfile** *Digestfile*

After copying the image, write the digest of the resulting image to the file.  (This option is not available with the remote Podman client)

#### **--disable-content-trust**

This is a Docker specific option to disable image verification to a Docker
registry and is not supported by Podman.  This flag is a NOOP and provided
solely for scripting compatibility.

#### **--format**, **-f**=*format*

Manifest Type (oci, v2s2, or v2s1) to use when pushing an image.

#### **--quiet**, **-q**

When writing the output image, suppress progress output

#### **--remove-signatures**

Discard any pre-existing signatures in the image. (This option is not available with the remote Podman client)

#### **--sign-by**=*key*

Add a signature at the destination using the specified key. (This option is not available with the remote Podman client)

#### **--tls-verify**=*true|false*

Require HTTPS and verify certificates when contacting registries (default: true). If explicitly set to true,
then TLS verification will be used. If set to false, then TLS verification will not be used. If not specified,
TLS verification will be used unless the target registry is listed as an insecure registry in registries.conf.

## EXAMPLE

This example pushes the image specified by the imageID to a local directory in docker format.

 `# podman push imageID dir:/path/to/image`

This example pushes the image specified by the imageID to a local directory in oci format.

 `# podman push imageID oci-archive:/path/to/layout:image:tag`

This example pushes the image specified by the imageID to a container registry named registry.example.com

 `# podman push imageID docker://registry.example.com/repository:tag`

This example pushes the image specified by the imageID to a container registry named registry.example.com and saves the digest in the specified digestfile.

 `# podman push --digestfile=/tmp/mydigest imageID docker://registry.example.com/repository:tag`

This example pushes the image specified by the imageID and puts it into the local docker container store

 `# podman push imageID docker-daemon:image:tag`

This example pushes the alpine image to umohnani/alpine on dockerhub and reads the creds from
the path given to --authfile

```
# podman push --authfile temp-auths/myauths.json alpine docker://docker.io/umohnani/alpine
Getting image source signatures
Copying blob sha256:5bef08742407efd622d243692b79ba0055383bbce12900324f75e56f589aedb0
 4.03 MB / 4.03 MB [========================================================] 1s
Copying config sha256:ad4686094d8f0186ec8249fc4917b71faa2c1030d7b5a025c29f26e19d95c156
 1.41 KB / 1.41 KB [========================================================] 1s
Writing manifest to image destination
Storing signatures
```

This example pushes the rhel7 image to rhel7-dir with the "oci" manifest type
```
# podman push --format oci registry.access.redhat.com/rhel7 dir:rhel7-dir
Getting image source signatures
Copying blob sha256:9cadd93b16ff2a0c51ac967ea2abfadfac50cfa3af8b5bf983d89b8f8647f3e4
 71.41 MB / 71.41 MB [======================================================] 9s
Copying blob sha256:4aa565ad8b7a87248163ce7dba1dd3894821aac97e846b932ff6b8ef9a8a508a
 1.21 KB / 1.21 KB [========================================================] 0s
Copying config sha256:f1b09a81455c351eaa484b61aacd048ab613c08e4c5d1da80c4c46301b03cf3b
 3.01 KB / 3.01 KB [========================================================] 0s
Writing manifest to image destination
Storing signatures
```

## SEE ALSO
podman(1), podman-pull(1), podman-login(1), containers-certs.d(5)
