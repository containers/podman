% podman-quadlet-install 1

## NAME
podman\-quadlet\-install - Install a quadlet file or quadlet application

## SYNOPSIS
**podman quadlet install** [*options*] *quadlet-path-or-url* [*files-path-or-url*]...

## DESCRIPTION

Install a Quadlet file or an application (which may include multiple Quadlet files) for the current user. You can specify Quadlet files as local files or web URLs.

This command allows you to:

    * Install a single Quadlet file, optionally followed by additional non-Quadlet files.

    * Specify a directory containing multiple Quadlet files and other non-Quadlet files for installation.

Note: If a quadlet is part of an application, removing that specific quadlet will remove the entire application. When a quadlet is installed from a directory, all files installed from that directory—including both quadlet and non-quadlet files—are considered part
of a single application.

## OPTIONS

#### **--reload-systemd**

Reload systemd after installing Quadlets (default true).

## EXAMPLES

Install quadlet from a file.

```
$ podman quadlet install /path/to/myquadlet.container
```
Install quadlet from a dir.

```
$ podman quadlet install /path/to/dir/
```

Install quadlet from a url
```
$ podman quadlet install https://github.com/containers/podman/blob/main/test/e2e/quadlet/basic.container
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-quadlet(1)](podman-quadlet.1.md)**
