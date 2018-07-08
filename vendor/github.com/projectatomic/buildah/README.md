![buildah logo](https://cdn.rawgit.com/projectatomic/buildah/master/logos/buildah-logo_large.png)

# [Buildah](https://www.youtube.com/embed/YVk5NgSiUw8) - a tool that facilitates building OCI container images

[![Go Report Card](https://goreportcard.com/badge/github.com/projectatomic/buildah)](https://goreportcard.com/report/github.com/projectatomic/buildah)
[![Travis](https://travis-ci.org/projectatomic/buildah.svg?branch=master)](https://travis-ci.org/projectatomic/buildah)

The Buildah package provides a command line tool that can be used to
* create a working container, either from scratch or using an image as a starting point
* create an image, either from a working container or via the instructions in a Dockerfile
* images can be built in either the OCI image format or the traditional upstream docker image format
* mount a working container's root filesystem for manipulation
* unmount a working container's root filesystem
* use the updated contents of a container's root filesystem as a filesystem layer to create a new image
* delete a working container or an image
* rename a local container

**[Buildah Demos](demos)**

**[Changelog](CHANGELOG.md)**

**[Contributing](CONTRIBUTING.md)**

**[Development Plan](developmentplan.md)**

**[Installation notes](install.md)**

**[Troubleshooting Guide](troubleshooting.md)**

**[Tutorials](docs/tutorials)**

## Example

From [`./examples/lighttpd.sh`](examples/lighttpd.sh):

```bash
$ cat > lighttpd.sh <<"EOF"
#!/bin/bash -x

ctr1=`buildah from ${1:-fedora}`

## Get all updates and install our minimal httpd server
buildah run $ctr1 -- dnf update -y
buildah run $ctr1 -- dnf install -y lighttpd

## Include some buildtime annotations
buildah config --annotation "com.example.build.host=$(uname -n)" $ctr1

## Run our server and expose the port
buildah config --cmd "/usr/sbin/lighttpd -D -f /etc/lighttpd/lighttpd.conf" $ctr1
buildah config --port 80 $ctr1

## Commit this container to an image name
buildah commit $ctr1 ${2:-$USER/lighttpd}
EOF

$ chmod +x lighttpd.sh
$ sudo ./lighttpd.sh
```

## Commands
| Command                                              | Description                                                                                          |
| ---------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| [buildah-add(1)](/docs/buildah-add.md)               | Add the contents of a file, URL, or a directory to the container.                                    |
| [buildah-bud(1)](/docs/buildah-bud.md)               | Build an image using instructions from Dockerfiles.                                                  |
| [buildah-commit(1)](/docs/buildah-commit.md)         | Create an image from a working container.                                                            |
| [buildah-config(1)](/docs/buildah-config.md)         | Update image configuration settings.                                                                 |
| [buildah-containers(1)](/docs/buildah-containers.md) | List the working containers and their base images.                                                   |
| [buildah-copy(1)](/docs/buildah-copy.md)             | Copies the contents of a file, URL, or directory into a container's working directory.               |
| [buildah-from(1)](/docs/buildah-from.md)             | Creates a new working container, either from scratch or using a specified image as a starting point. |
| [buildah-images(1)](/docs/buildah-images.md)         | List images in local storage.                                                                        |
| [buildah-inspect(1)](/docs/buildah-inspect.md)       | Inspects the configuration of a container or image.                                                  |
| [buildah-mount(1)](/docs/buildah-mount.md)           | Mount the working container's root filesystem.                                                       |
| [buildah-push(1)](/docs/buildah-push.md)             | Push an image from local storage to elsewhere.                                                       |
| [buildah-rename(1)](/docs/buildah-rename.md)         | Rename a local container.                                                                            |
| [buildah-rm(1)](/docs/buildah-rm.md)                 | Removes one or more working containers.                                                              |
| [buildah-rmi(1)](/docs/buildah-rmi.md)               | Removes one or more images.                                                                          |
| [buildah-run(1)](/docs/buildah-run.md)               | Run a command inside of the container.                                                               |
| [buildah-tag(1)](/docs/buildah-tag.md)               | Add an additional name to a local image.                                                             |
| [buildah-umount(1)](/docs/buildah-umount.md)         | Unmount a working container's root file system.                                                      |
| [buildah-unshare(1)](/docs/buildah-unshare.md)       | Launch a command in a user namespace with modified ID mappings.                                      |
| [buildah-version(1)](/docs/buildah-version.md)       | Display the Buildah Version Information                                                              |

**Future goals include:**
* more CI tests
* additional CLI commands (?)
