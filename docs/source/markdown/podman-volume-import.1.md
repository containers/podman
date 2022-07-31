% podman-volume-import(1)

## NAME
podman\-volume\-import - Import tarball contents into an existing podman volume

## SYNOPSIS
**podman volume import** *volume* [*source*]

## DESCRIPTION

**podman volume import** imports the contents of a tarball into the podman volume's mount point.
The contents of the volume will be merged with the content of the tarball with the latter taking precedence.
**podman volume import** can consume piped input when using `-` as source path.

The given volume must already exist and will not be created by podman volume import.

Note: Following command is not supported by podman-remote.

#### **--help**

Print usage statement

## EXAMPLES

```
$ gunzip -c hello.tar.gz | podman volume import myvol -
```
```
$ podman volume import myvol test.tar
```
```
$ podman volume export myvol | podman volume import oldmyvol -
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-volume(1)](podman-volume.1.md)**, **[podman-volume-export(1)](podman-volume-export.1.md)**
