% podman(1) podman-pull - Simple tool to pull an image from a registry
% Urvashi Mohnani
# podman-pull "1" "July 2017" "podman"

## NAME
podman-pull - Pull an image from a registry

## SYNOPSIS
**podman pull**
**NAME[:TAG|@DIGEST]**
[**--help**|**-h**]

## DESCRIPTION
Copies an image from a registry onto the local machine. **podman pull** pulls an
image from Docker Hub if a registry is not specified in the command line argument.
If an image tag is not specified, **podman pull** defaults to the image with the
**latest** tag (if it exists) and pulls it. **podman pull** can also pull an image
using its digest **podman pull [image]@[digest]**. **podman pull** can be used to pull
images from archives and local storage using different transports.

## imageID
Image stored in local container/storage

## SOURCE

 The SOURCE is a location to get container images
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

  **ostree:**_image_[**@**_/absolute/repo/path_]
  An image in local OSTree repository.  _/absolute/repo/path_ defaults to _/ostree/repo_.

**podman [GLOBAL OPTIONS]**

**podman pull [GLOBAL OPTIONS]**

**podman pull NAME[:TAG|@DIGEST]**

## OPTIONS

**--authfile**

Path of the authentication file. Default is ${XDG_RUNTIME\_DIR}/containers/auth.json, which is set using `podman login`.
If the authorization state is not found there, $HOME/.docker/config.json is checked, which is set using `docker login`.

**--cert-dir**

Pathname of a directory containing TLS certificates and keys

**--creds**

Credentials (USERNAME:PASSWORD) to use for authenticating to a registry

**--quiet, -q**

Suppress output information when pulling images

**--signature-policy="PATHNAME"**

Pathname of a signature policy file to use.  It is not recommended that this
option be used, as the default behavior of using the system-wide default policy
(frequently */etc/containers/policy.json*) is most often preferred

**--tls-verify**

Require HTTPS and verify certificates when contacting registries (default: true)

## EXAMPLES

```
# podman pull --signature-policy /etc/containers/policy.json alpine:latest
Trying to pull registry.access.redhat.com/alpine:latest... Failed
Trying to pull registry.fedoraproject.org/alpine:latest... Failed
Trying to pull docker.io/library/alpine:latest...Getting image source signatures
Copying blob sha256:88286f41530e93dffd4b964e1db22ce4939fffa4a4c665dab8591fbab03d4926
 1.90 MB / 1.90 MB [========================================================] 0s
Copying config sha256:76da55c8019d7a47c347c0dceb7a6591144d232a7dd616242a367b8bed18ecbc
 1.48 KB / 1.48 KB [========================================================] 0s
Writing manifest to image destination
Storing signatures
```

```
# podman pull --authfile temp-auths/myauths.json docker://docker.io/umohnani/finaltest
Trying to pull docker.io/umohnani/finaltest:latest...Getting image source signatures
Copying blob sha256:6d987f6f42797d81a318c40d442369ba3dc124883a0964d40b0c8f4f7561d913
 1.90 MB / 1.90 MB [========================================================] 0s
Copying config sha256:ad4686094d8f0186ec8249fc4917b71faa2c1030d7b5a025c29f26e19d95c156
 1.41 KB / 1.41 KB [========================================================] 0s
Writing manifest to image destination
Storing signatures
```

```
# podman pull --creds testuser:testpassword docker.io/umohnani/finaltest
Trying to pull docker.io/umohnani/finaltest:latest...Getting image source signatures
Copying blob sha256:6d987f6f42797d81a318c40d442369ba3dc124883a0964d40b0c8f4f7561d913
 1.90 MB / 1.90 MB [========================================================] 0s
Copying config sha256:ad4686094d8f0186ec8249fc4917b71faa2c1030d7b5a025c29f26e19d95c156
 1.41 KB / 1.41 KB [========================================================] 0s
Writing manifest to image destination
Storing signatures
```

```
# podman pull --tls-verify=false --cert-dir image/certs docker.io/umohnani/finaltest
Trying to pull docker.io/umohnani/finaltest:latest...Getting image source signatures
Copying blob sha256:6d987f6f42797d81a318c40d442369ba3dc124883a0964d40b0c8f4f7561d913
 1.90 MB / 1.90 MB [========================================================] 0s
Copying config sha256:ad4686094d8f0186ec8249fc4917b71faa2c1030d7b5a025c29f26e19d95c156
 1.41 KB / 1.41 KB [========================================================] 0s
Writing manifest to image destination
Storing signatures
```

## SEE ALSO
podman(1), podman-push(1), crio(8), crio.conf(5), docker-login(1)

## HISTORY
July 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
