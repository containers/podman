![KPOD logo](https://cdn.rawgit.com/kubernetes-incubator/cri-o/master/logo/crio-logo.svg)
# libpod - library for running OCI-based containers in Pods

### Status: Development

## What is the scope of this project?

libpod provides a library for applications looking to use the Container Pod concept popularized by Kubernetes.
libpod also contains a tool kpod, which allows you to manage Pods, Containers, and Container Images.

At a high level, we expect the scope of libpod/kpod to the following functionalities:

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
| Command                                              | Description                                                               | Demo|
| ---------------------------------------------------- | --------------------------------------------------------------------------|-----|
| [kpod(1)](/docs/kpod.1.md)                           | Simple management tool for pods and images                                ||
| [kpod-attach(1)](/docs/kpod-attach.1.md)             | Attach to a running container.
| [kpod-cp(1)](/docs/kpod-cp.1.md)                     | Instead of providing a `kpod cp` command, the man page `kpod-cp` describes how to use the `kpod mount` command to have even more flexibility and functionality.||
| [kpod-diff(1)](/docs/kpod-diff.1.md)                 | Inspect changes on a container or image's filesystem                      |[![...](/docs/play.png)](https://asciinema.org/a/FXfWB9CKYFwYM4EfqW3NSZy1G)|
| [kpod-exec(1)](/docs/kpod-exec.1.md)                 | Execute a command in a running container.
| [kpod-export(1)](/docs/kpod-export.1.md)             | Export container's filesystem contents as a tar archive                   |[![...](/docs/play.png)](https://asciinema.org/a/913lBIRAg5hK8asyIhhkQVLtV)|
| [kpod-history(1)](/docs/kpod-history.1.md)           | Shows the history of an image                                             |[![...](/docs/play.png)](https://asciinema.org/a/bCvUQJ6DkxInMELZdc5DinNSx)|
| [kpod-images(1)](/docs/kpod-images.1.md)             | List images in local storage                                              |[![...](/docs/play.png)](https://asciinema.org/a/133649)|
| [kpod-info(1)](/docs/kpod-info.1.md)                 | Display system information                                                ||
| [kpod-inspect(1)](/docs/kpod-inspect.1.md)           | Display the configuration of a container or image                         |[![...](/docs/play.png)](https://asciinema.org/a/133418)|
| [kpod-kill(1)](/docs/kpod-kill.1.md)                 | Kill the main process in one or more running containers                   |[![...](/docs/play.png)](https://asciinema.org/a/3jNos0A5yzO4hChu7ddKkUPw7)|
| [kpod-load(1)](/docs/kpod-load.1.md)                 | Load an image from docker archive or oci                                  |[![...](/docs/play.png)](https://asciinema.org/a/kp8kOaexEhEa20P1KLZ3L5X4g)|
| [kpod-login(1)](/docs/kpod-login.1.md)               | Login to a container registry						   |[![...](/docs/play.png)](https://asciinema.org/a/oNiPgmfo1FjV2YdesiLpvihtV)|
| [kpod-logout(1)](/docs/kpod-logout.1.md)             | Logout of a container registry                                            |[![...](/docs/play.png)](https://asciinema.org/a/oNiPgmfo1FjV2YdesiLpvihtV)|
| [kpod-logs(1)](/docs/kpod-logs.1.md)                 | Display the logs of a container                                           ||
| [kpod-mount(1)](/docs/kpod-mount.1.md)               | Mount a working container's root filesystem                               ||
| [kpod-pause(1)](/docs/kpod-pause.1.md)               | Pause one or more running containers                                      |[![...](/docs/play.png)](https://asciinema.org/a/141292)|
| [kpod-ps(1)](/docs/kpod-ps.1.md)                     | Prints out information about containers                                   |[![...](/docs/play.png)](https://asciinema.org/a/bbT41kac6CwZ5giESmZLIaTLR)|
| [kpod-pull(1)](/docs/kpod-pull.1.md)                 | Pull an image from a registry                                             |[![...](/docs/play.png)](https://asciinema.org/a/lr4zfoynHJOUNu1KaXa1dwG2X)|
| [kpod-push(1)](/docs/kpod-push.1.md)                 | Push an image to a specified destination                                  |[![...](/docs/play.png)](https://asciinema.org/a/133276)|
| [kpod-rename(1)](/docs/kpod-rename.1.md)             | Rename a container                                                        ||
| [kpod-rm(1)](/docs/kpod-rm.1.md)                     | Removes one or more containers                                            |[![...](/docs/play.png)](https://asciinema.org/a/7EMk22WrfGtKWmgHJX9Nze1Qp)|
| [kpod-rmi(1)](/docs/kpod-rmi.1.md)                   | Removes one or more images                                                |[![...](/docs/play.png)](https://asciinema.org/a/133799)|
| [kpod-save(1)](/docs/kpod-save.1.md)                 | Saves an image to an archive                                              |[![...](/docs/play.png)](https://asciinema.org/a/kp8kOaexEhEa20P1KLZ3L5X4g)|
| [kpod-start(1)](/docs/kpod-start.1.md)               | Starts one or more containers
| [kpod-stats(1)](/docs/kpod-stats.1.md)               | Display a live stream of one or more containers' resource usage statistics||
| [kpod-stop(1)](/docs/kpod-stop.1.md)                 | Stops one or more running containers                                      ||
| [kpod-tag(1)](/docs/kpod-tag.1.md)                   | Add an additional name to a local image                                   |[![...](/docs/play.png)](https://asciinema.org/a/133803)|
| [kpod-umount(1)](/docs/kpod-umount.1.md)             | Unmount a working container's root filesystem                             ||
| [kpod-unpause(1)](/docs/kpod-unpause.1.md)           | Unpause one or more running containers                                    |[![...](/docs/play.png)](https://asciinema.org/a/141292)|
| [kpod-version(1)](/docs/kpod-version.1.md)           | Display the version information                                           |[![...](/docs/play.png)](https://asciinema.org/a/mfrn61pjZT9Fc8L4NbfdSqfgu)|
| [kpod-wait(1)](/docs/kpod-wait.1.md)                 | Wait on one or more containers to stop and print their exit codes||

## OCI Hooks Support

[KPOD configures OCI Hooks to run when launching a container](./hooks.md)

## KPOD Usage Transfer

[Useful information for ops and dev transfer as it relates to infrastructure that utilizes KPOD](/transfer.md)

## Communication

For async communication and long running discussions please use issues and pull requests on the github repo. This will be the best place to discuss design and implementation.

For sync communication we have an IRC channel #KPOD, on chat.freenode.net, that everyone is welcome to join and chat about development.

## [Installation Instructions](install.md)


### Current Roadmap

1. Basic pod/container lifecycle, basic image pull (done)
1. Support for tty handling and state management (done)
1. Basic integration with kubelet once client side changes are ready (done)
1. Support for log management, networking integration using CNI, pluggable image/storage management (done)
1. Support for exec/attach (done)
