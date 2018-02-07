% podman(1) podman-push - Push an image from local storage to elsewhere
% Dan Walsh
# podman-push "1" "June 2017" "podman"

## NAME
podman push - Push an image from local storage to elsewhere

## SYNOPSIS
**podman** **push** [*options* [...]] **imageID** [**destination**]

## DESCRIPTION
Pushes an image from local storage to a specified destination.
Push is mainly used to push images to registries, however **podman push**
can be used to save images to tarballs and directories using the following
transports: **dir:**, **docker-archive:**, **docker-daemon:**, **oci-archive:**, and **ostree:**.

## imageID
Image stored in local container/storage

## DESTINATION

 The DESTINATION is a location to store container images
 The Image "DESTINATION" uses a "transport":"details" format.
 If a transport is not given, podman push will attempt to push
 to a registry.

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

## OPTIONS

**--authfile**

Path of the authentication file. Default is ${XDG_RUNTIME\_DIR}/containers/auth.json, which is set using `podman login`.
If the authorization state is not found there, $HOME/.docker/config.json is checked, which is set using `docker login`.

**--creds="CREDENTIALS"**

Credentials (USERNAME:PASSWORD) to use for authenticating to a registry

**cert-dir="PATHNAME"**

Pathname of a directory containing TLS certificates and keys.
Default certificates directory is _/etc/containers/certs.d_.

**--compress**

Compress tarball image layers when pushing to a directory using the 'dir' transport. (default is same compression type, compressed or uncompressed, as source)
Note: This flag can only be set when using the **dir** transport

**--format, -f**

Manifest Type (oci, v2s1, or v2s2) to use when pushing an image to a directory using the 'dir:' transport (default is manifest type of source)
Note: This flag can only be set when using the **dir** transport

**--quiet, -q**

When writing the output image, suppress progress output

**--remove-signatures**

Discard any pre-existing signatures in the image

**--signature-policy="PATHNAME"**

Pathname of a signature policy file to use.  It is not recommended that this
option be used, as the default behavior of using the system-wide default policy
(frequently */etc/containers/policy.json*) is most often preferred

**--sign-by="KEY"**

Add a signature at the destination using the specified key

**--tls-verify**

Require HTTPS and verify certificates when contacting registries (default: true)

## EXAMPLE

This example extracts the imageID image to a local directory in docker format.

 `# podman push imageID dir:/path/to/image`

This example extracts the imageID image to a local directory in oci format.

 `# podman push imageID oci-archive:/path/to/layout:image:tag`

This example extracts the imageID image to a container registry named registry.example.com

 `# podman push imageID docker://registry.example.com/repository:tag`

This example extracts the imageID image and puts into the local docker container store

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
podman(1), podman-pull(1), crio(8), crio.conf(5), docker-login(1)
