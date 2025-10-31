% podman-quadlet-install 1

## NAME
podman\-quadlet\-install - Install a quadlet file or quadlet application

## SYNOPSIS
**podman quadlet install** [*options*] *quadlet-path-or-url* [*files-path-or-url*]...

## DESCRIPTION

Install a Quadlet file or an application (which may include multiple Quadlet files) for the current user. You can specify Quadlet files as local files or web URLs.

This command allows you to:

    * Install a single Quadlet file, optionally followed by additional non-Quadlet files.

    * Specify a directory containing multiple Quadlet files and other non-Quadlet files for installation ( example a config file for a quadlet container ).

    * Install multiple Quadlets from a single file where each Quadlet is separated by a `---` delimiter. When using multiple quadlets in a single file, each quadlet section must include a `# FileName=<name>` comment to specify the name for that quadlet.

Note: If a quadlet is part of an application, removing that specific quadlet will remove the entire application. When a quadlet is installed from a directory, all files installed from that directory—including both quadlet and non-quadlet files—are considered part of a single application.

Note: In case user wants to install Quadlet application then first path should be the path to application directory.

## OPTIONS

#### **--reload-systemd**

Reload systemd after installing Quadlets (default true).
In order to disable it users need to manually set the value
of this flag to `false`.

#### **--replace**, **-r**

Replace the Quadlet installation even if the generated unit file already exists (default false).
In order to enable it, users need to manually set the value
of this flag to `true`. This flag is used primarily to update an existing unit.

## EXAMPLES

Install quadlet from a file.

```
$ podman quadlet install test-service-quadlet.container
/home/user/.config/containers/systemd/test-service-quadlet.container
```

Install quadlet from a dir.

```
$ podman quadlet install /home/user/work/quadlet-app/
/home/user/.config/containers/systemd/myquadlet1.container
/home/user/.config/containers/systemd/myquadlet2.container
/install/path/myquadlet1.container
/install/path/myquadlet2.container
```

Install quadlet from a url
```
$ podman quadlet install https://github.com/containers/podman/blob/main/test/e2e/quadlet/basic.container
/home/user/.config/containers/systemd/basic.container
```

Install multiple quadlets from a single file
```
$ cat webapp.quadlets
# FileName=web-server
[Container]
Image=nginx:latest
ContainerName=web-server
PublishPort=8080:80

---

# FileName=app-storage
[Volume]
Label=app=webapp

---

# FileName=app-network
[Network]
Subnet=10.0.0.0/24

$ podman quadlet install webapp.quadlets
/home/user/.config/containers/systemd/web-server.container
/home/user/.config/containers/systemd/app-storage.volume
/home/user/.config/containers/systemd/app-network.network
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-quadlet(1)](podman-quadlet.1.md)**
