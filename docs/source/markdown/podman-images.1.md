% podman-images(1)

## NAME
podman\-images - List images in local storage

## SYNOPSIS
**podman images** [*options*]

**podman image list** [*options*]

**podman image ls** [*options*]

## DESCRIPTION
Displays locally stored images, their names, and their IDs.

## OPTIONS

#### **--all**, **-a**

Show all images (by default filter out the intermediate image layers). The default is false.

#### **--digests**

Show image digests

#### **--filter**=*filter*, **-f**

Provide filter values.

The *filters* argument format is of `key=value` or `key!=value`. If there is more than one *filter*, then pass multiple OPTIONS: **--filter** *foo=bar* **--filter** *bif=baz*.

Supported filters:

| Filter             | Description                                                                                   |
| :----------------: | --------------------------------------------------------------------------------------------- |
| *id*               | Filter by image id.                                                                           |
| *before*           | Filter by images created before the given IMAGE (name or tag).                                |
| *containers*       | Filter by images with a running container.                                                    |
| *dangling*         | Filter by dangling (unused) images.                                                           |
| *intermediate*     | Filter by images that are dangling and have no children                                       |
| *label*            | Filter by images with (or without, in the case of label!=[...] is used) the specified labels. |
| *manifest*         | Filter by images that are manifest lists.                                                     |
| *readonly*         | Filter by read-only or read/write images.                                                     |
| *reference*        | Filter by image name.                                                                         |
| *after*/*since*    | Filter by images created after the given IMAGE (name or tag).                                 |
| *until*            | Filter by images created until the given duration or time.                                    |

The `id` *filter* accepts the image id string.

The `before` *filter* accepts formats: `<image-name>[:<tag>]`, `<image id>` or `<image@digest>`.

The `containers` *filter* shows images that have a running container based on that image.

The `dangling` *filter* shows images that are taking up disk space and serve no purpose. Dangling image is a file system layer that was used in a previous build of an image and is no longer referenced by any image. They are denoted with the `<none>` tag, consume disk space and serve no active purpose.

The `intermediate` *filter* shows images that are dangling and have no children.

The `label` *filter* accepts two formats. One is the `label`=*key* or `label`=*key*=*value*, which shows images with the specified labels. The other format is the `label!`=*key* or `label!`=*key*=*value*, which shows images without the specified labels.

The `manifest` *filter* shows images that are manifest lists.

The `readonly` *filter* shows, as a default, both read-only and read/write images. Read-only images can be configured by modifying the  `additionalimagestores` in the `/etc/containers/storage.conf` file.

The `reference` *filter* accepts the pattern of an image reference `<image-name>[:<tag>]`.

The `after` or `since` *filter* accepts formats: `<image-name>[:<tag>]`, `<image id>` or `<image@digest>`.

The `until` *filter* accepts formats: golang duration, RFC3339 time, or a Unix timestamp and shows all images that are created until that time.

#### **--format**=*format*

Change the default output format.  This can be of a supported type like 'json'
or a Go template.
Valid placeholders for the Go template are listed below:

| **Placeholder** | **Description**                                                               |
| --------------- | ----------------------------------------------------------------------------- |
| .ID             | Image ID                                                                      |
| .Repository     | Image repository                                                              |
| .Tag            | Image tag                                                                     |
| .Digest         | Image digest                                                                  |
| .CreatedSince   | Elapsed time since the image was created			     					  |
| .CreatedAt      | Time when the image was created                                               |
| .Size           | Size of layer on disk                                                         |
| .History        | History of the image layer                                                    |

#### **--history**

Display the history of image names.  If an image gets re-tagged or untagged, then the image name history gets prepended (latest image first).  This is especially useful when undoing a tag operation or an image does not contain any name because it has been untagged.

#### **--no-trunc**

Do not truncate the output (default *false*).

#### **--noheading**, **-n**

Omit the table headings from the listing of images.

#### **--quiet**, **-q**

Lists only the image IDs.

#### **--sort**=*sort*=*created*

Sort by created, id, repository, size or tag (default: created)

## EXAMPLE

```
# podman images
REPOSITORY                                   TAG      IMAGE ID       CREATED       SIZE
docker.io/kubernetes/pause                   latest   e3d42bcaf643   3 years ago   251 kB
<none>                                       <none>   ebb91b73692b   4 weeks ago   27.2 MB
docker.io/library/ubuntu                     latest   4526339ae51c   6 weeks ago   126 MB
```

```
# podman images --quiet
e3d42bcaf643
ebb91b73692b
4526339ae51c
```

```
# podman images --noheading
docker.io/kubernetes/pause                   latest   e3d42bcaf643   3 years ago   251 kB
<none>                                       <none>   ebb91b73692b   4 weeks ago   27.2 MB
docker.io/library/ubuntu                     latest   4526339ae51c   6 weeks ago   126 MB
```

```
# podman images --no-trunc
REPOSITORY                                   TAG      IMAGE ID                                                                  CREATED       SIZE
docker.io/kubernetes/pause                   latest   sha256:e3d42bcaf643097dd1bb0385658ae8cbe100a80f773555c44690d22c25d16b27   3 years ago   251 kB
<none>                                       <none>   sha256:ebb91b73692bd27890685846412ae338d13552165eacf7fcd5f139bfa9c2d6d9   4 weeks ago   27.2 MB
docker.io/library/ubuntu                     latest   sha256:4526339ae51c3cdc97956a7a961c193c39dfc6bd9733b0d762a36c6881b5583a   6 weeks ago   126 MB
```

```
# podman images --format "table {{.ID}} {{.Repository}} {{.Tag}}"
IMAGE ID       REPOSITORY                                   TAG
e3d42bcaf643   docker.io/kubernetes/pause                   latest
ebb91b73692b   <none>                                       <none>
4526339ae51c   docker.io/library/ubuntu                     latest
```

```
# podman images --filter dangling=true
REPOSITORY   TAG      IMAGE ID       CREATED       SIZE
<none>       <none>   ebb91b73692b   4 weeks ago   27.2 MB
```

```
# podman images --format json
[
    {
	"id": "e3d42bcaf643097dd1bb0385658ae8cbe100a80f773555c44690d22c25d16b27",
	"names": [
	    "docker.io/kubernetes/pause:latest"
	],
	"digest": "sha256:0aecf73ff86844324847883f2e916d3f6984c5fae3c2f23e91d66f549fe7d423",
	"created": "2014-07-19T07:02:32.267701596Z",
	"size": 250665
    },
    {
	"id": "ebb91b73692bd27890685846412ae338d13552165eacf7fcd5f139bfa9c2d6d9",
	"names": [
	    "\u003cnone\u003e"
	],
	"digest": "sha256:ba7e4091d27e8114a205003ca6a768905c3395d961624a2c78873d9526461032",
	"created": "2017-10-26T03:07:22.796184288Z",
	"size": 27170520
    },
    {
	"id": "4526339ae51c3cdc97956a7a961c193c39dfc6bd9733b0d762a36c6881b5583a",
	"names": [
	    "docker.io/library/ubuntu:latest"
	],
	"digest": "sha256:193f7734ddd68e0fb24ba9af8c2b673aecb0227b026871f8e932dab45add7753",
	"created": "2017-10-10T20:59:05.10196344Z",
	"size": 126085200
    }
]
```

```
# podman images --sort repository
REPOSITORY                                   TAG      IMAGE ID       CREATED       SIZE
<none>                                      <none>   2460217d76fc   About a minute ago   4.41 MB
docker.io/library/alpine                    latest   3fd9065eaf02   5 months ago         4.41 MB
localhost/myapp                             latest   b2e0ad03474a   About a minute ago   4.41 MB
registry.access.redhat.com/rhel7            latest   7a840db7f020   2 weeks ago          211 MB
registry.fedoraproject.org/fedora           27       801894bc0e43   6 weeks ago          246 MB
```

```
# podman images
REPOSITORY                 TAG      IMAGE ID       CREATED         SIZE
localhost/test             latest   18f0c080cd72   4 seconds ago   4.42 MB
docker.io/library/alpine   latest   3fd9065eaf02   5 months ago    4.41 MB
# podman images -a
REPOSITORY                 TAG      IMAGE ID       CREATED         SIZE
localhost/test             latest   18f0c080cd72   6 seconds ago   4.42 MB
<none>                     <none>   270e70dc54c0   7 seconds ago   4.42 MB
<none>                     <none>   4ed6fbe43414   8 seconds ago   4.41 MB
<none>                     <none>   6b0df8e71508   8 seconds ago   4.41 MB
docker.io/library/alpine   latest   3fd9065eaf02   5 months ago    4.41 MB
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[containers-storage.conf(5)](https://github.com/containers/storage/blob/main/docs/containers-storage.conf.5.md)**

## HISTORY
March 2017, Originally compiled by Dan Walsh `<dwalsh@redhat.com>`
