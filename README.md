![PODMAN logo](https://cdn.rawgit.com/kubernetes-incubator/cri-o/master/logo/crio-logo.svg)
# libpod - library for running OCI-based containers in Pods

### Status: Development

## What is the scope of this project?

libpod provides a library for applications looking to use the Container Pod concept popularized by Kubernetes.
libpod also contains a tool podman, which allows you to manage Pods, Containers, and Container Images.

At a high level, we expect the scope of libpod/podman to the following functionalities:

* Support multiple image formats including the existing Docker/OCI image formats
* Support for multiple means to download images including trust & image verification
* Container image management (managing image layers, overlay filesystems, etc)
* Container and POD process lifecycle management
* Resource isolation of containers and PODS.

## What is not in scope for this project?

* Building container images. See Buildah
* Signing and pushing images to various image storages.  See Skopeo.
* Container Runtimes daemons for working with Kubernetes CRIs  See CRI-O.

The plan is to use OCI projects and best of breed libraries for different aspects:
- Runtime: [runc](https://github.com/opencontainers/runc) (or any OCI runtime-spec implementation) and [oci runtime tools](https://github.com/opencontainers/runtime-tools)
- Images: Image management using [containers/image](https://github.com/containers/image)
- Storage: Storage and management of image layers using [containers/storage](https://github.com/containers/storage)
- Networking: Networking support through use of [CNI](https://github.com/containernetworking/cni)

libpod is currently in active development.

## Commands
| Command                                                  | Description                                                               | Demo|
| :------------------------------------------------------- | :------------------------------------------------------------------------ | :----|
| [podman(1)](/docs/podman.1.md)                           | Simple management tool for pods and images                                ||
| [podman-attach(1)](/docs/podman-attach.1.md)             | Attach to a running container                                             ||
| [podman-commit(1)](/docs/podman-commit.1.md)             | Create new image based on the changed container                           ||
| [podman-cp(1)](/docs/podman-cp.1.md)                     | Instead of providing a `podman cp` command, the man page `podman-cp` describes how to use the `podman mount` command to have even more flexibility and functionality||
| [podman-create(1)](/docs/podman-create.1.md)             | Create a new container                                                    ||
| [podman-diff(1)](/docs/podman-diff.1.md)                 | Inspect changes on a container or image's filesystem                      |[![...](/docs/play.png)](https://asciinema.org/a/FXfWB9CKYFwYM4EfqW3NSZy1G)|
| [podman-exec(1)](/docs/podman-exec.1.md)                 | Execute a command in a running container
| [podman-export(1)](/docs/podman-export.1.md)             | Export container's filesystem contents as a tar archive                   |[![...](/docs/play.png)](https://asciinema.org/a/913lBIRAg5hK8asyIhhkQVLtV)|
| [podman-history(1)](/docs/podman-history.1.md)           | Shows the history of an image                                             |[![...](/docs/play.png)](https://asciinema.org/a/bCvUQJ6DkxInMELZdc5DinNSx)|
| [podman-images(1)](/docs/podman-images.1.md)             | List images in local storage                                              |[![...](/docs/play.png)](https://asciinema.org/a/133649)|
| [podman-import(1)](/docs/podman-import.1.md)             | Import a tarball and save it as a filesystem image                        ||
| [podman-info(1)](/docs/podman-info.1.md)                 | Display system information                                                ||
| [podman-inspect(1)](/docs/podman-inspect.1.md)           | Display the configuration of a container or image                         |[![...](/docs/play.png)](https://asciinema.org/a/133418)|
| [podman-kill(1)](/docs/podman-kill.1.md)                 | Kill the main process in one or more running containers                   |[![...](/docs/play.png)](https://asciinema.org/a/3jNos0A5yzO4hChu7ddKkUPw7)|
| [podman-load(1)](/docs/podman-load.1.md)                 | Load an image from docker archive or oci                                  |[![...](/docs/play.png)](https://asciinema.org/a/kp8kOaexEhEa20P1KLZ3L5X4g)|
| [podman-login(1)](/docs/podman-login.1.md)               | Login to a container registry						   |[![...](/docs/play.png)](https://asciinema.org/a/oNiPgmfo1FjV2YdesiLpvihtV)|
| [podman-logout(1)](/docs/podman-logout.1.md)             | Logout of a container registry                                            |[![...](/docs/play.png)](https://asciinema.org/a/oNiPgmfo1FjV2YdesiLpvihtV)|
| [podman-logs(1)](/docs/podman-logs.1.md)                 | Display the logs of a container                                           ||
| [podman-mount(1)](/docs/podman-mount.1.md)               | Mount a working container's root filesystem                               ||
| [podman-pause(1)](/docs/podman-pause.1.md)               | Pause one or more running containers                                      |[![...](/docs/play.png)](https://asciinema.org/a/141292)|
| [podman-ps(1)](/docs/podman-ps.1.md)                     | Prints out information about containers                                   |[![...](/docs/play.png)](https://asciinema.org/a/bbT41kac6CwZ5giESmZLIaTLR)|
| [podman-pull(1)](/docs/podman-pull.1.md)                 | Pull an image from a registry                                             |[![...](/docs/play.png)](https://asciinema.org/a/lr4zfoynHJOUNu1KaXa1dwG2X)|
| [podman-push(1)](/docs/podman-push.1.md)                 | Push an image to a specified destination                                  |[![...](/docs/play.png)](https://asciinema.org/a/133276)|
| [podman-rm(1)](/docs/podman-rm.1.md)                     | Removes one or more containers                                            |[![...](/docs/play.png)](https://asciinema.org/a/7EMk22WrfGtKWmgHJX9Nze1Qp)|
| [podman-rmi(1)](/docs/podman-rmi.1.md)                   | Removes one or more images                                                |[![...](/docs/play.png)](https://asciinema.org/a/133799)|
| [podman-save(1)](/docs/podman-save.1.md)                 | Saves an image to an archive                                              |[![...](/docs/play.png)](https://asciinema.org/a/kp8kOaexEhEa20P1KLZ3L5X4g)|
| [podman-start(1)](/docs/podman-start.1.md)               | Starts one or more containers
| [podman-stats(1)](/docs/podman-stats.1.md)               | Display a live stream of one or more containers' resource usage statistics||
| [podman-stop(1)](/docs/podman-stop.1.md)                 | Stops one or more running containers                                      ||
| [podman-tag(1)](/docs/podman-tag.1.md)                   | Add an additional name to a local image                                   |[![...](/docs/play.png)](https://asciinema.org/a/133803)|
| [podman-top(1)](/docs/podman-top.1.md)                   | Display the running processes of a container
| [podman-umount(1)](/docs/podman-umount.1.md)             | Unmount a working container's root filesystem                             ||
| [podman-unpause(1)](/docs/podman-unpause.1.md)           | Unpause one or more running containers                                    |[![...](/docs/play.png)](https://asciinema.org/a/141292)|
| [podman-version(1)](/docs/podman-version.1.md)           | Display the version information                                           |[![...](/docs/play.png)](https://asciinema.org/a/mfrn61pjZT9Fc8L4NbfdSqfgu)|
| [podman-wait(1)](/docs/podman-wait.1.md)                 | Wait on one or more containers to stop and print their exit codes||

## OCI Hooks Support

[PODMAN configures OCI Hooks to run when launching a container](./hooks.md)

## PODMAN Usage Transfer

[Useful information for ops and dev transfer as it relates to infrastructure that utilizes PODMAN](/transfer.md)

## Communication

For async communication and long running discussions please use issues and pull requests on the github repo. This will be the best place to discuss design and implementation.

For sync communication we have an IRC channel #PODMAN, on chat.freenode.net, that everyone is welcome to join and chat about development.

## [Installation Instructions](install.md)


### Current Roadmap

1. Basic pod/container lifecycle, basic image pull (done)
1. Support for tty handling and state management (done)
1. Basic integration with kubelet once client side changes are ready (done)
1. Support for log management, networking integration using CNI, pluggable image/storage management (done)
1. Support for exec/attach (done)
