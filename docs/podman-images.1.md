% podman(1) podman-images - List images in local storage
% Dan Walsh
# podman-images "1" "March 2017" "podman"

## NAME
podman images - List images in local storage

## SYNOPSIS
**podman** **images** [*options* [...]]

## DESCRIPTION
Displays locally stored images, their names, and their IDs.

## OPTIONS

**--digests**

Show image digests

**--filter, -f=[]**

Filter output based on conditions provided (default [])

**--format**

Change the default output format.  This can be of a supported type like 'json'
or a Go template.

**--noheading, -n**

Omit the table headings from the listing of images.

**--no-trunc, --notruncate**

Do not truncate output.

**--quiet, -q**

Lists only the image IDs.


## EXAMPLE

```
# podman images
REPOSITORY                                   TAG      IMAGE ID       CREATED       SIZE
docker.io/kubernetes/pause                   latest   e3d42bcaf643   3 years ago   251kB
<none>                                       <none>   ebb91b73692b   4 weeks ago   27.2MB
docker.io/library/ubuntu                     latest   4526339ae51c   6 weeks ago   126MB
```

```
# podman images --quiet
e3d42bcaf643
ebb91b73692b
4526339ae51c
```

```
# podman images --noheading
docker.io/kubernetes/pause                   latest   e3d42bcaf643   3 years ago   251kB
<none>                                       <none>   ebb91b73692b   4 weeks ago   27.2MB
docker.io/library/ubuntu                     latest   4526339ae51c   6 weeks ago   126MB
```

```
# podman images --no-trunc
REPOSITORY                                   TAG      IMAGE ID                                                                  CREATED       SIZE
docker.io/kubernetes/pause                   latest   sha256:e3d42bcaf643097dd1bb0385658ae8cbe100a80f773555c44690d22c25d16b27   3 years ago   251kB
<none>                                       <none>   sha256:ebb91b73692bd27890685846412ae338d13552165eacf7fcd5f139bfa9c2d6d9   4 weeks ago   27.2MB
docker.io/library/ubuntu                     latest   sha256:4526339ae51c3cdc97956a7a961c193c39dfc6bd9733b0d762a36c6881b5583a   6 weeks ago   126MB
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
<none>       <none>   ebb91b73692b   4 weeks ago   27.2MB
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

## SEE ALSO
podman(1)

## HISTORY
March 2017, Originally compiled by Dan Walsh <dwalsh@redhat.com>
