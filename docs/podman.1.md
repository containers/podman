% podman(1) podman - Simple management tool for pods and images
% Dan Walsh
# podman "1" "September 2016" "podman"
## NAME
podman - Simple management tool for containers and images

## SYNOPSIS
**podman** [*options*] COMMAND

# DESCRIPTION
podman is a simple client only tool to help with debugging issues when daemons
such as CRI runtime and the kubelet are not responding or failing. A shared API
layer could be created to share code between the daemon and podman. podman does not
require any daemon running. podman utilizes the same underlying components that
crio uses i.e. containers/image, container/storage, oci-runtime-tool/generate,
runc or any other OCI compatible runtime. podman shares state with crio and so
has the capability to debug pods/images created by crio.

**podman [GLOBAL OPTIONS]**

## GLOBAL OPTIONS

**--help, -h**
  Print usage statement

**--config value, -c**=**"config.file"**
   Path of a config file detailing container server configuration options

**--cpu-profile**
   Path to where the cpu performance results should be written

**--log-level**
   log messages above specified level: debug, info, warn, error (default), fatal or panic

**--root**=**value**
   Path to the root directory in which data, including images, is stored

**--runroot**=**value**
   Path to the 'run directory' where all state information is stored

**--runtime**=**value**
    Path to the OCI compatible binary used to run containers

**--storage-driver, -s**=**value**
   Select which storage driver is used to manage storage of images and containers (default is overlay)

**--storage-opt**=**value**
   Used to pass an option to the storage driver

**--version, -v**
  Print the version

## COMMANDS

| Command                                   | Description                                                                    |
| ----------------------------------------- | ------------------------------------------------------------------------------ |
| [podman-attach(1)](podman-attach.1.md)    | Attach to a running container.                                                 |
| [podman-build(1)](podman-build.1.md)      | Build a container using a Dockerfile.                                          |
| [podman-commit(1)](podman-commit.1.md)    | Create new image based on the changed container.                               |
| [podman-cp(1)](podman-cp.1.md)            | Copy files/folders between a container and the local filesystem.               |
| [podman-create(1)](podman-create.1.md)    | Create a new container.                                                        |
| [podman-diff(1)](podman-diff.1.md)        | Inspect changes on a container or image's filesystem.                          |
| [podman-exec(1)](podman-exec.1.md)        | Execute a command in a running container.                                      |
| [podman-export(1)](podman-export.1.md)    | Export a container's filesystem contents as a tar archive.                     |
| [podman-history(1)](podman-history.1.md)  | Show the history of an image.                                                  |
| [podman-images(1)](podman-images.1.md)    | List images in local storage.                                                  |
| [podman-import(1)](podman-import.1.md)    | Import a tarball and save it as a filesystem image.                            |
| [podman-info(1)](podman-info.1.md)        | Displays Podman related system information.                                    |
| [podman-inspect(1)](podman-inspect.1.md)  | Display a container or image's configuration.                                  |
| [podman-kill(1)](podman-kill.1.md)        | Kill the main process in one or more containers.                               |
| [podman-load(1)](podman-load.1.md)        | Load an image from the docker archive.                                         |
| [podman-login(1)](podman-login.1.md)      | Login to a container registry.                                                 |
| [podman-logout(1)](podman-logout.1.md)    | Logout of a container registry.                                                |
| [podman-logs(1)](podman-logs.1.md)        | Display the logs of a container.                                               |
| [podman-mount(1)](podman-mount.1.md)      | Mount a working container's root filesystem.                                   |
| [podman-pause(1)](podman-pause.1.md)      | Pause one or more containers.                                                  |
| [podman-port(1)](podman-port.1.md)        | List port mappings for the container.                                          |
| [podman-ps(1)](podman-ps.1.md)            | Prints out information about containers.                                       |
| [podman-pull(1)](podman-pull.1.md)        | Pull an image from a registry.                                                 |
| [podman-push(1)](podman-push.1.md)        | Push an image from local storage to elsewhere.                                 |
| [podman-rm(1)](podman-rm.1.md)            | Remove one or more containers.                                                 |
| [podman-rmi(1)](podman-rmi.1.md)          | Removes one or more locally stored images.                                     |
| [podman-run(1)](podman-run.1.md)          | Run a command in a container.                                                  |
| [podman-save(1)](podman-save.1.md)        | Save an image to docker-archive or oci.                                        |
| [podman-search(1)](podman-search.1.md)    | Search a registry for an image.                                                |
| [podman-start(1)](podman-start.1.md)      | Starts one or more containers.                                                 |
| [podman-stats(1)](podman-stats.1.md)      | Display a live stream of one or more container's resource usage statistics.    |
| [podman-stop(1)](podman-stop.1.md)        | Stop one or more running containers.                                           |
| [podman-tag(1)](podman-tag.1.md)          | Add an additional name to a local image.                                       |
| [podman-top(1)](podman-top.1.md)          | Display the running processes of a container.                                  |
| [podman-umount(1)](podman-umount.1.md)    | Unmount a working container's root filesystem.                                 |
| [podman-unpause(1)](podman-unpause.1.md)  | Unpause one or more containers.                                                |
| [podman-version(1)](podman-version.1.md)  | Display the Podman version information.                                        |
| [podman-wait(1)](podman-wait.1.md)        | Wait on one or more containers to stop and print their exit codes.             |

## SEE ALSO
crio(8), crio.conf(5)

## HISTORY
Dec 2016, Originally compiled by Dan Walsh <dwalsh@redhat.com>
