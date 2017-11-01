% kpod(1) kpod-inspect - Display a container or image's configuration
% Dan Walsh
# kpod-inspect "1" "July 2017" "kpod"

## NAME
kpod inspect - Display a container or image's configuration

## SYNOPSIS
**kpod** **inspect** [*options* [...]] name

## DESCRIPTION
This displays the low-level information on containers and images identified by name or ID. By default, this will render all results in a JSON array. If the container and image have the same name, this will return container JSON for unspecified type. If a format is specified, the given template will be executed for each result.

## OPTIONS

**--type, t="TYPE"**

Return data on items of the specified type.  Type can be 'container', 'image' or 'all' (default: all)

**--format, -f="FORMAT"**

Format the output using the given Go template

**--size**

Display the total file size if the type is a container


## EXAMPLE

kpod inspect redis:alpine

{
    "ArgsEscaped": true,
    "AttachStderr": false,
    "AttachStdin": false,
    "AttachStdout": false,
    "Cmd": [
        "/bin/sh",
        "-c",
        "#(nop) ",
        "CMD [\"redis-server\"]"
    ],
    "Domainname": "",
    "Entrypoint": [
        "entrypoint.sh"
    ],
    "Env": [
        "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
        "REDIS_VERSION=3.2.9",
        "REDIS_DOWNLOAD_URL=http://download.redis.io/releases/redis-3.2.9.tar.gz",
        "REDIS_DOWNLOAD_SHA=6eaacfa983b287e440d0839ead20c2231749d5d6b78bbe0e0ffa3a890c59ff26"
    ],
    "ExposedPorts": {
        "6379/tcp": {}
    },
    "Hostname": "e1ede117fb1e",
    "Image": "sha256:75e877aa15b534396de82d385386cc4dda7819d5cbb018b9f97b77aeb8f4b55a",
    "Labels": {},
    "OnBuild": [],
    "OpenStdin": false,
    "StdinOnce": false,
    "Tty": false,
    "User": "",
    "Volumes": {
        "/data": {}
    },
    "WorkingDir": "/data"
}
{
    "ID": "b3f2436bdb978c1d33b1387afb5d7ba7e3243ed2ce908db431ac0069da86cb45",
    "Names": [
        "docker.io/library/redis:alpine"
    ],
    "Digests": [
        "sha256:88286f41530e93dffd4b964e1db22ce4939fffa4a4c665dab8591fbab03d4926",
        "sha256:07b1ac6c7a5068201d8b63a09bb15358ec1616b813ef3942eb8cc12ae191227f",
        "sha256:91e2e140ea27b3e89f359cd9fab4ec45647dda2a8e5fb0c78633217d9dca87b5",
        "sha256:08957ceaa2b3be874cde8d7fa15c274300f47185acd62bca812a2ffb6228482d",
        "sha256:acd3d12a6a79f772961a771f678c1a39e1f370e7baeb9e606ad8f1b92572f4ab",
        "sha256:4ad88df090801e8faa8cf0be1f403b77613d13e11dad73f561461d482f79256c",
        "sha256:159ac12c79e1a8d85dfe61afff8c64b96881719139730012a9697f432d6b739a"
    ],
    "Parent": "",
    "Comment": "",
    "Created": "2017-06-28T22:14:36.35280993Z",
    "Container": "ba8d6c6b0d7fdd201fce404236136b44f3bfdda883466531a3d1a1f87906770b",
    "ContainerConfig": {
        "Hostname": "e1ede117fb1e",
        "Domainname": "",
        "User": "",
        "AttachStdin": false,
        "AttachStdout": false,
        "AttachStderr": false,
        "Tty": false,
        "OpenStdin": false,
        "StdinOnce": false,
        "Env": [
            "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
            "REDIS_VERSION=3.2.9",
            "REDIS_DOWNLOAD_URL=http://download.redis.io/releases/redis-3.2.9.tar.gz",
            "REDIS_DOWNLOAD_SHA=6eaacfa983b287e440d0839ead20c2231749d5d6b78bbe0e0ffa3a890c59ff26"
        ],
        "Cmd": [
            "/bin/sh",
            "-c",
            "#(nop) ",
            "CMD [\"redis-server\"]"
        ],
        "ArgsEscaped": true,
        "Image": "sha256:75e877aa15b534396de82d385386cc4dda7819d5cbb018b9f97b77aeb8f4b55a",
        "Volumes": {
            "/data": {}
        },
        "WorkingDir": "/data",
        "Entrypoint": [
            "entrypoint.sh"
        ],
        "Labels": {},
        "OnBuild": []
    },
    "Author": "",
    "Config": {
        "ExposedPorts": {
            "6379/tcp": {}
        },
        "Env": [
            "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
            "REDIS_VERSION=3.2.9",
            "REDIS_DOWNLOAD_URL=http://download.redis.io/releases/redis-3.2.9.tar.gz",
            "REDIS_DOWNLOAD_SHA=6eaacfa983b287e440d0839ead20c2231749d5d6b78bbe0e0ffa3a890c59ff26"
        ],
        "Entrypoint": [
            "entrypoint.sh"
        ],
        "Cmd": [
            "redis-server"
        ],
        "Volumes": {
            "/data": {}
        },
        "WorkingDir": "/data"
    },
    "Architecture": "amd64",
    "OS": "linux",
    "Size": 3965955,
    "VirtualSize": 19808086,
    "GraphDriver": {
        "Name": "overlay",
        "Data": {
            "MergedDir": "/var/lib/containers/storage/overlay/2059d805c90e034cb773d9722232ef018a72143dd31113b470fb876baeccd700/merged",
            "UpperDir": "/var/lib/containers/storage/overlay/2059d805c90e034cb773d9722232ef018a72143dd31113b470fb876baeccd700/diff",
            "WorkDir": "/var/lib/containers/storage/overlay/2059d805c90e034cb773d9722232ef018a72143dd31113b470fb876baeccd700/work"
        }
    },
    "RootFS": {
        "type": "layers",
        "diff_ids": [
            "sha256:5bef08742407efd622d243692b79ba0055383bbce12900324f75e56f589aedb0",
            "sha256:c92a8fc997217611d0bfc9ff14d7ec00350ca564aef0ecbf726624561d7872d7",
            "sha256:d4c406dea37a107b0cccb845611266a146725598be3e82ba31c55c08d1583b5a",
            "sha256:8b4fa064e2b6c03a6c37089b0203f167375a8b49259c0ad7cb47c8c1e58b3fa0",
            "sha256:c393e3d0b00ddf6b4166f1e2ad68245e08e9e3be0a0567a36d0a43854f03bfd6",
            "sha256:38047b4117cb8bb3bba82991daf9a4e14ba01f9f66c1434d4895a7e96f67d8ba"
        ]
    }
}


## SEE ALSO
kpod(1)
