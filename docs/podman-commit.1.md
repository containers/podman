% podman-commit(1)

## NAME
podman\-commit - Create new image based on the changed container

## SYNOPSIS
**podman commit** [*options*] *container* *image*

## DESCRIPTION
`podman commit` creates an image based on a changed container. The author of the
image can be set using the `--author` flag. Various image instructions can be
configured with the `--change` flag and a commit message can be set using the
`--message` flag. The container and its processes are paused while the image is
committed. This minimizes the likelihood of data corruption when creating the new
image. If this is not desired, the `--pause` flag can be set to false. When the commit
is complete, podman will print out the ID of the new image.

If *image* does not begin with a registry name component, `localhost` will be added to the name.

## OPTIONS

**--author, -a**

Set the author for the committed image

**--change, -c**

Apply the following possible instructions to the created image:
**CMD** | **ENTRYPOINT** | **ENV** | **EXPOSE** | **LABEL** | **ONBUILD** | **STOPSIGNAL** | **USER** | **VOLUME** | **WORKDIR**

Can be set multiple times

**--format, -f**

Set the format of the image manifest and metadata.  The currently supported formats are _oci_ and _docker_.  If
not specifically set, the default format used is _oci_.

**--iidfile** *ImageIDfile*

Write the image ID to the file.

**--message, -m**

Set commit message for committed image.  The message field is not supported in _oci_ format.

**--pause, -p**

Pause the container when creating an image

**--quiet, -q**

Suppress output

## EXAMPLES

```
$ podman commit --change CMD=/bin/bash --change ENTRYPOINT=/bin/sh --change LABEL=blue=image reverent_golick image-commited
Getting image source signatures
Copying blob sha256:b41deda5a2feb1f03a5c1bb38c598cbc12c9ccd675f438edc6acd815f7585b86
 25.80 MB / 25.80 MB [======================================================] 0s
Copying config sha256:c16a6d30f3782288ec4e7521c754acc29d37155629cb39149756f486dae2d4cd
 448 B / 448 B [============================================================] 0s
Writing manifest to image destination
Storing signatures
e3ce4d93051ceea088d1c242624d659be32cf1667ef62f1d16d6b60193e2c7a8
```

```
$ podman commit -q --message "committing container to image" reverent_golick image-commited
e3ce4d93051ceea088d1c242624d659be32cf1667ef62f1d16d6b60193e2c7a8
```

```
$ podman commit -q --author "firstName lastName" reverent_golick image-commited
e3ce4d93051ceea088d1c242624d659be32cf1667ef62f1d16d6b60193e2c7a8
```

```
$ podman commit -q --pause=false reverent_golick image-commited
e3ce4d93051ceea088d1c242624d659be32cf1667ef62f1d16d6b60193e2c7a8
```

## SEE ALSO
podman(1), podman-run(1), podman-create(1)

## HISTORY
December 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
