% podman-play(1)

## NAME
podman\-play - Play containers, pods or volumes based on a structured input file

## SYNOPSIS
**podman play** *subcommand*

## DESCRIPTION
The play command will recreate containers, pods or volumes based on the input from a structured (like YAML)
file input.  Containers will be automatically started.

## COMMANDS

| Command  | Man Page                                            | Description                                                                  |
| -------  | --------------------------------------------------- | ---------------------------------------------------------------------------- |
| kube     | [podman-play-kube(1)](podman-play-kube.1.md)        | Create containers, pods or volumes based on Kubernetes YAML.                         |

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-pod(1)](podman-pod.1.md)**, **[podman-container(1)](podman-container.1.md)**, **[podman-generate(1)](podman-generate.1.md)**, **[podman-play-kube(1)](podman-play-kube.1.md)**
