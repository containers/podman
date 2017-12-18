% podman(1) podman-inspect - Display a container or image's configuration
% Dan Walsh
# podman-inspect "1" "July 2017" "podman"

## NAME
podman inspect - Display a container or image's configuration

## SYNOPSIS
**podman** **inspect** [*options* [...]] name

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

```
# podman inspect fedora
{
    "Id": "422dc563ca3260ad9ef5c47a1c246f5065d7f177ce51f4dd208efd82967ff182",
    "Digest": "sha256:1b9bfb4e634dc1e5c19d0fa1eb2e5a28a5c2b498e3d3e4ac742bd7f5dae08611",
    "RepoTags": [
        "docker.io/library/fedora:latest"
    ],
    "RepoDigests": [
        "docker.io/library/fedora@sha256:1b9bfb4e634dc1e5c19d0fa1eb2e5a28a5c2b498e3d3e4ac742bd7f5dae08611"
    ],
    "Parent": "",
    "Comment": "",
    "Created": "2017-11-14T21:07:08.475840838Z",
    "Config": {
        "Env": [
            "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
            "DISTTAG=f27container",
            "FGC=f27",
            "FBR=f27"
        ]
    },
    "Version": "17.06.2-ce",
    "Author": "[Adam Miller \u003cmaxamillion@fedoraproject.org\u003e] [Patrick Uiterwijk \u003cpatrick@puiterwijk.org\u003e]",
    "Architecture": "amd64",
    "Os": "linux",
    "Size": 251722732,
    "VirtualSize": 514895140,
    "GraphDriver": {
        "Name": "overlay",
        "Data": {
            "MergedDir": "/var/lib/containers/storage/overlay/d32459d9ce237564fb93573b85cbc707600d43fbe5e46e8eeef22cad914bb516/merged",
            "UpperDir": "/var/lib/containers/storage/overlay/d32459d9ce237564fb93573b85cbc707600d43fbe5e46e8eeef22cad914bb516/diff",
            "WorkDir": "/var/lib/containers/storage/overlay/d32459d9ce237564fb93573b85cbc707600d43fbe5e46e8eeef22cad914bb516/work"
        }
    },
    "RootFS": {
        "Type": "layers",
        "Layers": [
            "sha256:d32459d9ce237564fb93573b85cbc707600d43fbe5e46e8eeef22cad914bb516"
        ]
    },
    "Labels": null,
    "Annotations": {}
}
```

## SEE ALSO
podman(1)

## HISTORY
July 2017, Originally compiled by Dan Walsh <dwalsh@redhat.com>
