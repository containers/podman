% podman(1) podman-commit - Tool to create new image based on the changed container
% Urvashi Mohnani
# podman-commit "1" "December 2017" "podman"

## NAME
podman commit - Create new image based on the changed container

## SYNOPSIS
**podman commit**
**TARBALL**
[**--author**|**-a**]
[**--change**|**-c**]
[**--message**|**-m**]
[**--help**|**-h**]

## DESCRIPTION
**podman commit** creates an image based on a changed container. The author of the
image can be set using the **--author** flag. Various image instructions can be
configured with the **--change** flag and a commit message can be set using the
**--message** flag. The container and its processes are paused while the image is
committed. This minimizes the likelihood of data corruption when creating the new
image. If this is not desired, the **--pause** flag can be set to false.

**podman [GLOBAL OPTIONS]**

**podman commit [GLOBAL OPTIONS]**

**podman commit [OPTIONS] CONTAINER**

## OPTIONS

**--author, -a**
Set the author for the committed image

**--change, -c**
Apply the following possible instructions to the created image:
**CMD** | **ENTRYPOINT** | **ENV** | **EXPOSE** | **LABEL** | **STOPSIGNAL** | **USER** | **VOLUME** | **WORKDIR**
Can be set multiple times

**--message, -m**
Set commit message for committed image

**--pause, -p**
Pause the container when creating an image

## EXAMPLES

```
# podman commit --change CMD=/bin/bash --change ENTRYPOINT=/bin/sh --change LABEL=blue=image reverent_golick image-commited
Getting image source signatures
Copying blob sha256:b41deda5a2feb1f03a5c1bb38c598cbc12c9ccd675f438edc6acd815f7585b86
 25.80 MB / 25.80 MB [======================================================] 0s
Copying config sha256:c16a6d30f3782288ec4e7521c754acc29d37155629cb39149756f486dae2d4cd
 448 B / 448 B [============================================================] 0s
Writing manifest to image destination
Storing signatures
```

```
# podman commit --message "commiting container to image" reverent_golick image-commited
Getting image source signatures
Copying blob sha256:b41deda5a2feb1f03a5c1bb38c598cbc12c9ccd675f438edc6acd815f7585b86
 25.80 MB / 25.80 MB [======================================================] 0s
Copying config sha256:af376cdda5c0ac1d9592bf56567253d203f8de6a8edf356c683a645d75221540
 376 B / 376 B [============================================================] 0s
Writing manifest to image destination
Storing signatures
```

```
# podman commit --author "firstName lastName" reverent_golick
Getting image source signatures
Copying blob sha256:b41deda5a2feb1f03a5c1bb38c598cbc12c9ccd675f438edc6acd815f7585b86
 25.80 MB / 25.80 MB [======================================================] 0s
Copying config sha256:d61387b4d5edf65edee5353e2340783703074ffeaaac529cde97a8357eea7645
 378 B / 378 B [============================================================] 0s
Writing manifest to image destination
Storing signatures
```

```
# podman commit --pause=false reverent_golick image-commited
Getting image source signatures
Copying blob sha256:b41deda5a2feb1f03a5c1bb38c598cbc12c9ccd675f438edc6acd815f7585b86
 25.80 MB / 25.80 MB [======================================================] 0s
Copying config sha256:5813fe8a3b18696089fd09957a12e88bda43dc1745b5240879ffffe93240d29a
 419 B / 419 B [============================================================] 0s
Writing manifest to image destination
Storing signatures
```

## SEE ALSO
podman(1), podman-run(1), podman-create(1)

## HISTORY
December 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
