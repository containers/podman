% podman-container(1)

## NAME
podman\-container - Manage containers

## SYNOPSIS
**podman container** *subcommand*

## DESCRIPTION
The container command allows you to manage containers

## COMMANDS

| Command  | Man Page                                            | Description                                                                  |
| -------  | --------------------------------------------------- | ---------------------------------------------------------------------------- |
| attach   | [podman-attach(1)](podman-attach.1.md)              | Attach to a running container.                                               |
| checkpoint | [podman-container-checkpoint(1)](podman-container-checkpoint.1.md)  | Checkpoints one or more containers.                        |
| cleanup  | [podman-container-cleanup(1)](podman-container-cleanup.1.md)    | Cleanup containers network and mountpoints.                               |
| commit   | [podman-commit(1)](podman-commit.1.md)              | Create new image based on the changed container.                             |
| create   | [podman-create(1)](podman-create.1.md)              | Create a new container.                                                      |
| diff     | [podman-diff(1)](podman-diff.1.md)                  | Inspect changes on a container or image's filesystem.                        |
| exec     | [podman-exec(1)](podman-exec.1.md)                  | Execute a command in a running container.                                    |
| export   | [podman-export(1)](podman-export.1.md)              | Export a container's filesystem contents as a tar archive.                   |
| inspect  | [podman-inspect(1)](podman-inspect.1.md)            | Display a container or image's configuration.                                |
| kill     | [podman-kill(1)](podman-kill.1.md)                  | Kill the main process in one or more containers.                             |
| logs     | [podman-logs(1)](podman-logs.1.md)                  | Display the logs of a container.                                             |
| ls       | [podman-ps(1)](podman-ps.1.md)                      | Prints out information about containers.                                     |
| mount    | [podman-mount(1)](podman-mount.1.md)                | Mount a working container's root filesystem.                                 |
| pause    | [podman-pause(1)](podman-pause.1.md)                | Pause one or more containers.                                                |
| port     | [podman-port(1)](podman-port.1.md)                  | List port mappings for the container.                                        |
| refresh  | [podman-refresh(1)](podman-container-refresh.1.md)  | Refresh the state of all containers                                          |
| restart  | [podman-restart(1)](podman-restart.1.md)            | Restart one or more containers.                                              |
| restore  | [podman-container-restore(1)](podman-container-restore.1.md)  | Restores one or more containers from a checkpoint.                 |
| rm       | [podman-rm(1)](podman-rm.1.md)                      | Remove one or more containers.                                               |
| run      | [podman-run(1)](podman-run.1.md)                    | Run a command in a container.                                                |
| start    | [podman-start(1)](podman-start.1.md)                | Starts one or more containers.                                               |
| stats    | [podman-stats(1)](podman-stats.1.md)                | Display a live stream of one or more container's resource usage statistics.  |
| stop     | [podman-stop(1)](podman-stop.1.md)                  | Stop one or more running containers.                                         |
| top      | [podman-top(1)](podman-top.1.md)                    | Display the running processes of a container.                                |
| umount   | [podman-umount(1)](podman-umount.1.md)              | Unmount a working container's root filesystem.                               |
| unmount  | [podman-umount(1)](podman-umount.1.md)              | Unmount a working container's root filesystem.                               |
| unpause  | [podman-unpause(1)](podman-unpause.1.md)            | Unpause one or more containers.                                              |
| wait     | [podman-wait(1)](podman-wait.1.md)                  | Wait on one or more containers to stop and print their exit codes.           |

## SEE ALSO
podman, podman-exec, podman-run
