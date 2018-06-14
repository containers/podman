% podman-inspect "1"

## NAME
podman\-inspect - Display a container or image's configuration

## SYNOPSIS
**podman** **inspect** [*options* [...]] name [...]

## DESCRIPTION
This displays the low-level information on containers and images identified by name or ID. By default, this will render
all results in a JSON array. If the container and image have the same name, this will return container JSON for
unspecified type. If a format is specified, the given template will be executed for each result.

## OPTIONS

**--type, t="TYPE"**

Return JSON for the specified type.  Type can be 'container', 'image' or 'all' (default: all)

**--format, -f="FORMAT"**

Format the output using the given Go template.
The keys of the returned JSON can be used as the values for the --format flag (see examples below).

**--latest, -l**

Instead of providing the container name or ID, use the last created container. If you use methods other than Podman
to run containers such as CRI-O, the last started container could be from either of those methods.

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
    "ContainerConfig": {
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

```
# podman inspect a04 --format "{{.ImageName}}"
fedora
```

```
# podman inspect a04 --format "{{.GraphDriver.Name}}"
overlay
```

```
# podman inspect --format "size: {{.Size}}" alpine
size:   4405240
```

## SEE ALSO
podman(1)

## HISTORY
July 2017, Originally compiled by Dan Walsh <dwalsh@redhat.com>
