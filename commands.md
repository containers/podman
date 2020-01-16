![PODMAN logo](logo/podman-logo-source.svg)

# libpod - library for running OCI-based containers in Pods

## Podman Commands

| Command                                                                  | Description                                                                | Demo                                                                        | Script                                                                              |
| :----------------------------------------------------------------------- | :------------------------------------------------------------------------- | :-------------------------------------------------------------------------- | :---------------------------------------------------------------------------------- |
| [podman(1)](/docs/source/markdown/podman.1.md)                                           | Simple management tool for pods and images                                 |
| [podman-attach(1)](/docs/source/markdown/podman-attach.1.md)                             | Attach to a running container                                              |
| [podman-build(1)](/docs/source/markdown/podman-build.1.md)                               | Build an image using instructions from Dockerfiles                         |
| [podman-commit(1)](/docs/source/markdown/podman-commit.1.md)                             | Create new image based on the changed container                            |
| [podman-container(1)](/docs/source/markdown/podman-container.1.md)                       | Manage Containers                                                          |
| [podman-container-checkpoint(1)](/docs/source/markdown/podman-container-checkpoint.1.md) | Checkpoints one or more running containers                                 |
| [podman-container-cleanup(1)](/docs/source/markdown/podman-container-cleanup.1.md)       | Cleanup Container storage and networks                                     |
| [podman-container-exists(1)](/docs/source/markdown/podman-container-exists.1.md)         | Check if an container exists in local storage                              |
| [podman-container-prune(1)](/docs/source/markdown/podman-container-prune.1.md)           | Remove all stopped containers                                              |
| [podman-container-refresh(1)](/docs/source/markdown/podman-container-refresh.1.md)       | Refresh all containers state in database                                   |
| [podman-container-restore(1)](/docs/source/markdown/podman-container-restore.1.md)       | Restores one or more running containers                                    |
| [podman-container-runlabel(1)](/docs/source/markdown/podman-container-runlabel.1.md)     | Execute Image Label Method                                                 |
| [podman-cp(1)](/docs/source/markdown/podman-cp.1.md)                                     | Copy files/folders between a container and the local filesystem            |
| [podman-create(1)](/docs/source/markdown/podman-create.1.md)                             | Create a new container                                                     |
| [podman-diff(1)](/docs/source/markdown/podman-diff.1.md)                                 | Inspect changes on a container or image's filesystem                       |
| [podman-events(1)](/docs/source/markdown/podman-events.1.md)                             | Monitor Podman events                                                      |
| [podman-exec(1)](/docs/source/markdown/podman-exec.1.md)                                 | Execute a command in a running container                                   |
| [podman-export(1)](/docs/source/markdown/podman-export.1.md)                             | Export container's filesystem contents as a tar archive                    |
| [podman-generate(1)](/docs/source/markdown/podman-generate.1.md)                         | Generate structured output based on Podman containers and pods             |
| [podman-generate-kube(1)](/docs/source/markdown/podman-generate-kube.1.md)               | Generate Kubernetes YAML based on a container or Pod                       |
| [podman-generate-systemd(1)](/docs/source/markdown/podman-generate-systemd.1.md)         | Generate a Systemd unit file for a container                               |
| [podman-history(1)](/docs/source/markdown/podman-history.1.md)                           | Shows the history of an image                                              |
| [podman-image(1)](/docs/source/markdown/podman-image.1.md)                               | Manage Images                                                              |
| [podman-image-exists(1)](/docs/source/markdown/podman-image-exists.1.md)                 | Check if an image exists in local storage                                  |
| [podman-image-prune(1)](/docs/source/markdown/podman-image-prune.1.md)                   | Remove all unused images                                                   |
| [podman-image-sign(1)](/docs/source/markdown/podman-image-sign.1.md)                     | Create a signature for an image                                            |
| [podman-image-trust(1)](/docs/source/markdown/podman-image-trust.1.md)                   | Manage container registry image trust policy                               |
| [podman-images(1)](/docs/source/markdown/podman-images.1.md)                             | List images in local storage                                               | [![...](/docs/source/markdown/play.png)](https://podman.io/asciinema/podman/images/)        | [Here](https://github.com/containers/Demos/blob/master/podman_cli/podman_images.sh) |
| [podman-import(1)](/docs/source/markdown/podman-import.1.md)                             | Import a tarball and save it as a filesystem image                         |
| [podman-info(1)](/docs/source/markdown/podman-info.1.md)                                 | Display system information                                                 |
| [podman-init(1)](/docs/source/markdown/podman-init.1.md)                                 | Initialize a container                                                     |
| [podman-inspect(1)](/docs/source/markdown/podman-inspect.1.md)                           | Display the configuration of a container or image                          | [![...](/docs/source/markdown/play.png)](https://podman.io/asciinema/podman/inspect/)         | [Here](https://github.com/containers/Demos/blob/master/podman_cli/podman_inspect.sh) |
| [podman-kill(1)](/docs/source/markdown/podman-kill.1.md)                                 | Kill the main process in one or more running containers                    |
| [podman-load(1)](/docs/source/markdown/podman-load.1.md)                                 | Load an image from a container image archive                               |
| [podman-login(1)](/docs/source/markdown/podman-login.1.md)                               | Login to a container registry                                              |
| [podman-logout(1)](/docs/source/markdown/podman-logout.1.md)                             | Logout of a container registry                                             |
| [podman-logs(1)](/docs/source/markdown/podman-logs.1.md)                                 | Display the logs of a container                                            |
| [podman-mount(1)](/docs/source/markdown/podman-mount.1.md)                               | Mount a working container's root filesystem                                |
| [podman-network(1)](/docs/source/markdown/podman-network.1.md)                           | Manage Podman CNI networks             |
| [podman-network-create(1)](/docs/source/markdown/podman-network-create.1.md)             | Create a CNI network             |
| [podman-network-inspect(1)](/docs/source/markdown/podman-network-inspect.1.md)           | Inspect one or more Podman networks             |
| [podman-network-ls(1)](/docs/source/markdown/podman-network-ls.1.md)                     | Display a summary of Podman networks             |
| [podman-network-rm(1)](/docs/source/markdown/podman-network-rm.1.md)                     | Remove one or more Podman networks             |
| [podman-pause(1)](/docs/source/markdown/podman-pause.1.md)                               | Pause one or more running containers                                       | [![...](/docs/source/markdown/play.png)](https://podman.io/asciinema/podman/pause_unpause/)        | [Here](https://github.com/containers/Demos/blob/master/podman_cli/podman_pause_unpause.sh) |
| [podman-play(1)](/docs/source/markdown/podman-play.1.md)                                 | Play pods and containers based on a structured input file                  |
| [podman-pod(1)](/docs/source/markdown/podman-pod.1.md)                                   | Simple management tool for groups of containers, called pods               |
| [podman-pod-create(1)](/docs/source/markdown/podman-pod-create.1.md)                     | Create a new pod                                                           |
| [podman-pod-inspect(1)](/docs/source/markdown/podman-pod-inspect.1.md)                   | Inspect a pod                                                              |
| [podman-pod-kill(1)](podman-pod-kill.1.md)                               | Kill the main process of each container in pod.                            |
| [podman-pod-ps(1)](/docs/source/markdown/podman-pod-ps.1.md)                             | List the pods on the system                                                |
| [podman-pod-pause(1)](podman-pod-pause.1.md)                             | Pause one or more pods.                                                    |
| [podman-pod-restart](/docs/source/markdown/podman-pod-restart.1.md)                      | Restart one or more pods                                                   |
| [podman-pod-rm(1)](/docs/source/markdown/podman-pod-rm.1.md)                             | Remove one or more pods                                                    |
| [podman-pod-start(1)](/docs/source/markdown/podman-pod-start.1.md)                       | Start one or more pods                                                     |
| [podman-pod-stats(1)](/docs/source/markdown/podman-pod-stats.1.md)                       | Display a live stream of one or more pods' resource usage statistics       |                                                                             |                                                                                     |
| [podman-pod-stop(1)](/docs/source/markdown/podman-pod-stop.1.md)                         | Stop one or more pods                                                      |
| [podman-pod-top(1)](/docs/source/markdown/podman-pod-top.1.md)                           | Display the running processes of a pod                                     |
| [podman-pod-unpause(1)](podman-pod-unpause.1.md)                         | Unpause one or more pods.                                                  |
| [podman-port(1)](/docs/source/markdown/podman-port.1.md)                                 | List port mappings for running containers                                  |
| [podman-ps(1)](/docs/source/markdown/podman-ps.1.md)                                     | Prints out information about containers                                    |
| [podman-pull(1)](/docs/source/markdown/podman-pull.1.md)                                 | Pull an image from a registry                                              |
| [podman-push(1)](/docs/source/markdown/podman-push.1.md)                                 | Push an image to a specified destination                                   | [![...](/docs/source/markdown/play.png)](https://asciinema.org/a/133276)                    |
| [podman-restart](/docs/source/markdown/podman-restart.1.md)                              | Restarts one or more containers                                            | [![...](/docs/source/markdown/play.png)](https://asciinema.org/a/jiqxJAxcVXw604xdzMLTkQvHM) |
| [podman-rm(1)](/docs/source/markdown/podman-rm.1.md)                                     | Removes one or more containers                                             |
| [podman-rmi(1)](/docs/source/markdown/podman-rmi.1.md)                                   | Removes one or more images                                                 |
| [podman-run(1)](/docs/source/markdown/podman-run.1.md)                                   | Run a command in a container                                               |
| [podman-save(1)](/docs/source/markdown/podman-save.1.md)                                 | Saves an image to an archive                                               |
| [podman-service(1)](/docs/source/markdown/podman-service.1.md)                           | Run an API listening service |
| [podman-search(1)](/docs/source/markdown/podman-search.1.md)                             | Search a registry for an image                                             |
| [podman-start(1)](/docs/source/markdown/podman-start.1.md)                               | Starts one or more containers                                              |
| [podman-stats(1)](/docs/source/markdown/podman-stats.1.md)                               | Display a live stream of one or more containers' resource usage statistics |
| [podman-stop(1)](/docs/source/markdown/podman-stop.1.md)                                 | Stops one or more running containers                                       |
| [podman-system(1)](/docs/source/markdown/podman-system.1.md)                             | Manage podman                                                              |
| [podman-system-df(1)](/docs/source/markdown/podman-system-df.1.md)                       | Show podman disk usage.                                                    |
| [podman-system-info(1)](/docs/source/markdown/podman-info.1.md)                          | Displays Podman related system information.                                |
| [podman-system-migrate(1)](/docs/source/markdown/podman-system-migrate.1.md)             | Migrate existing containers to a new podman version.                       |
| [podman-system-prune(1)](/docs/source/markdown/podman-system-prune.1.md)                 | Remove all unused container, image and volume data.                        |
| [podman-system-renumber(1)](/docs/source/markdown/podman-system-renumber.1.md)           | Migrate lock numbers to handle a change in maximum number of locks.        |
| [podman-system-reset(1)](/docs/source/markdown/podman-system-reset.1.md)                 | Reset storage back to original state.  Remove all pods, containers, images, volumes. |
| [podman-tag(1)](/docs/source/markdown/podman-tag.1.md)                                   | Add an additional name to a local image                                    | [![...](/docs/source/markdown/play.png)](https://asciinema.org/a/133803)                    |
| [podman-top(1)](/docs/source/markdown/podman-top.1.md)                                   | Display the running processes of a container                               |
| [podman-umount(1)](/docs/source/markdown/podman-umount.1.md)                             | Unmount a working container's root filesystem                              |
| [podman-unpause(1)](/docs/source/markdown/podman-unpause.1.md)                           | Unpause one or more running containers                                     | [![...](/docs/source/markdown/play.png)](https://podman.io/asciinema/podman/pause_unpause/)        | [Here](https://github.com/containers/Demos/blob/master/podman_cli/podman_pause_unpause.sh) |
| [podman-unshare(1)](/docs/source/markdown/podman-unshare.1.md)                           | Run a command inside of a modified user namespace.                         |
| [podman-varlink(1)](/docs/source/markdown/podman-varlink.1.md)                           | Run the varlink backend                                                    |
| [podman-version(1)](/docs/source/markdown/podman-version.1.md)                           | Display the version information                                            |
| [podman-volume(1)](/docs/source/markdown/podman-volume.1.md)                             | Manage Volumes                                                             |
| [podman-volume-create(1)](/docs/source/markdown/podman-volume-create.1.md)               | Create a volume                                                            |
| [podman-volume-inspect(1)](/docs/source/markdown/podman-volume-inspect.1.md)             | Get detailed information on one or more volumes                            |
| [podman-volume-ls(1)](/docs/source/markdown/podman-volume-ls.1.md)                       | List all the available volumes                                             |
| [podman-volume-rm(1)](/docs/source/markdown/podman-volume-rm.1.md)                       | Remove one or more volumes                                                 |
| [podman-volume-prune(1)](/docs/source/markdown/podman-volume-prune.1.md)                 | Remove all unused volumes                                                  |
| [podman-wait(1)](/docs/source/markdown/podman-wait.1.md)                                 | Wait on one or more containers to stop and print their exit codes          |
