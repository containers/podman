# Building the Podman client on macOS

The following describes the process for building the Podman client on macOS.

## Install brew
Podman requires brew -- a package manager for macOS.  This will allow additional packages to be installed that are
needed by Podman.  See the [brew project page](https://brew.sh/) for installation instructions.

## Install build dependencies
Podman requires some software from brew to be able to build.  This can be done using brew from a macOS terminal:

```
$ brew install go go-md2man
```

## Obtain Podman source code

You can obtain the latest source code for Podman from its github repository.

```
$ git clone https://github.com/containers/podman go/src/github.com/containers/podman
```

## Build client
After completing the preparatory steps of obtaining the Podman source code and installing its dependencies, the client
can now be built.

```
$ cd go/src/github.com/containers/podman
$ make podman-remote
$ mv bin/darwin/podman bin/podman
```

The binary will be located in bin/
```
$ ls -l bin/
```

If you would like to build the docs associated with Podman on macOS:
```
$ make podman-remote-darwin-docs
$ ls docs/build/remote/darwin
```

To install and view these manpages:

```
$ cp -a docs/build/remote/darwin/* /usr/share/man/man1
$ man podman
```

## Using the client

To learn how to use the Podman client, refer its
[tutorial](https://github.com/containers/podman/blob/main/docs/tutorials/remote_client.md).
