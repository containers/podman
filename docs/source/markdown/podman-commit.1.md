% podman-commit 1

## NAME
podman\-commit - Create new image based on the changed container

## SYNOPSIS
**podman commit** [*options*] *container* [*image*]

**podman container commit** [*options*] *container* [*image*]

## DESCRIPTION
**podman commit** creates an image based on a changed *container*. The author of the image can be set using the **--author** OPTION. Various image instructions can be configured with the **--change** OPTION and a commit message can be set using the **--message** OPTION. The *container* and its processes aren't paused while the image is committed. If this is not desired, the **--pause** OPTION can be set to *true*. When the commit is complete, Podman prints out the ID of the new image.

If `image` does not begin with a registry name component, `localhost` is added to the name.
If `image` is not provided, the values for the `REPOSITORY` and `TAG` values of the created image is set to `<none>`.

## OPTIONS
#### **--author**, **-a**=*author*

Set the author for the committed image.

#### **--change**, **-c**=*instruction*

Apply the following possible instructions to the created image:

- *CMD*
- *ENTRYPOINT*
- *ENV*
- *EXPOSE*
- *LABEL*
- *ONBUILD*
- *STOPSIGNAL*
- *USER*
- *VOLUME*
- *WORKDIR*

Can be set multiple times.

#### **--format**, **-f**=**oci** | *docker*

Set the format of the image manifest and metadata.  The currently supported formats are **oci** and *docker*.\
The default is **oci**.

#### **--iidfile**=*ImageIDfile*

Write the image ID to the file.

#### **--include-volumes**

Include in the committed image any volumes added to the container by the **--volume** or **--mount** OPTIONS to the **[podman create](podman-create.1.md)** and **[podman run](podman-run.1.md)** commands.\
The default is **false**.

#### **--message**, **-m**=*message*

Set commit message for committed image.\
*IMPORTANT: The message field is not supported in `oci` format.*

#### **--pause**, **-p**

Pause the container when creating an image.\
The default is **false**.

#### **--quiet**, **-q**

Suppresses output.\
The default is **false**.

#### **--squash**, **-s**

Squash newly built layers into a single new layer.\
The default is **false**.

## EXAMPLES
Create image from container with entrypoint and label
```
$ podman commit --change CMD=/bin/bash --change ENTRYPOINT=/bin/sh --change "LABEL blue=image" reverent_golick image-committed
Getting image source signatures
Copying blob sha256:b41deda5a2feb1f03a5c1bb38c598cbc12c9ccd675f438edc6acd815f7585b86
 25.80 MB / 25.80 MB [======================================================] 0s
Copying config sha256:c16a6d30f3782288ec4e7521c754acc29d37155629cb39149756f486dae2d4cd
 448 B / 448 B [============================================================] 0s
Writing manifest to image destination
Storing signatures
e3ce4d93051ceea088d1c242624d659be32cf1667ef62f1d16d6b60193e2c7a8
```

Create image from container with commit message
```
$ podman commit -q --message "committing container to image"
reverent_golick image-committed
e3ce4d93051ceea088d1c242624d659be32cf1667ef62f1d16d6b60193e2c7a8
```

Create image from container with author
```
$ podman commit -q --author "firstName lastName" reverent_golick image-committed
e3ce4d93051ceea088d1c242624d659be32cf1667ef62f1d16d6b60193e2c7a8
```

Pause a running container while creating the image
```
$ podman commit -q --pause=true containerID image-committed
e3ce4d93051ceea088d1c242624d659be32cf1667ef62f1d16d6b60193e2c7a8
```

Create an image from a container with a default image tag
```
$ podman commit containerID
e3ce4d93051ceea088d1c242624d659be32cf1667ef62f1d16d6b60193e2c7a8
```

Create an image from container with default required capabilities are SETUID and SETGID
```
$ podman commit -q --change LABEL=io.containers.capabilities=setuid,setgid epic_nobel privimage
400d31a3f36dca751435e80a0e16da4859beb51ff84670ce6bdc5edb30b94066
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-run(1)](podman-run.1.md)**, **[podman-create(1)](podman-create.1.md)**

## HISTORY
December 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
