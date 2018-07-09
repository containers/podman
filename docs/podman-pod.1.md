% podman-pod "1"

## NAME
podman\-pod - Simple management tool for groups of containers, called pods.

## SYNOPSIS
**podman pod** *subcommand*

# DESCRIPTION
podman pod is a set of subcommands that manage pods, or groups of containers.

## SUBCOMMANDS

| Subcommand                                        | Description                                                                    |
| ------------------------------------------------- | ------------------------------------------------------------------------------ |
| [podman-pod-create(1)](podman-pod-create.1.md)    | Create a new pod.                                                              |
| [podman-pod-inspect(1)](podman-pod-inspect.1.md)  | Display a pod's configuration.                                                 |
| [podman-pod-kill(1)](podman-pod-kill.1.md)        | Kill the main process in one or more pods.                                     |
| [podman-pod-pause(1)](podman-pod-pause.1.md)      | Pause one or more pods.                                                        |
| [podman-pod-ps(1)](podman-pod-ps.1.md)            | Prints out information about pods.                                             |
| [podman-pod-restart(1)](podman-pod-restart.1.md)  | Restart one or more pods.                                                      |
| [podman-pod-rm(1)](podman-pod-rm.1.md)            | Remove one or more pods.                                                       |
| [podman-pod-start(1)](podman-pod-start.1.md)      | Starts one or more pods.                                                       |
| [podman-pod-stats(1)](podman-pod-stats.1.md)      | Display a live stream of one or more pod's resource usage statistics.          |
| [podman-pod-stop(1)](podman-pod-stop.1.md)        | Stop one or more running pods.                                                 |
| [podman-pod-top(1)](podman-pod-top.1.md)          | Display the running processes of a pod.                                        |
| [podman-pod-unpause(1)](podman-pod-unpause.1.md)  | Unpause one or more pods.                                                      |
| [podman-pod-wait(1)](podman-pod-wait.1.md)        | Wait on one or more pods to stop and print their exit codes.                   |

## HISTORY
Dec 2016, Originally compiled by Dan Walsh <dwalsh@redhat.com>
July 2018, Adapted from podman man page by Peter Hunt <pehunt@redhat.com>
