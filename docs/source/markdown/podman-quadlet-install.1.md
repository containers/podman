% podman-quadlet-install 1

## NAME
podman\-quadlet\-install - Install one or more quadlets

## SYNOPSIS
**podman quadlet install** [*options*] *path-or-url* [*path-or-url*]...

## DESCRIPTION

Install one or more Quadlets for the current user. Quadlets may be specified as local files, Web URLs, and OCI artifacts.

## OPTIONS

#### **--reload-systemd**

Reload systemd after installing Quadlets (default true).

## EXAMPLES

Install quadlet from a file.

```
$ podman quadlet install /path/to/myquadlet.container
```

Install quadlet from a url
```
$ podman quadlet install https://github.com/containers/podman/blob/main/test/e2e/quadlet/basic.container
```

Install quadlet from artifact
```
$ podman quadlet install oci-artifact://my-artifact:latest
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-quadlet(1)](podman-quadlet.1.md)**
