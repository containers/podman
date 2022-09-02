% podman-kube 1

## NAME
podman\-kube - Play containers, pods or volumes based on a structured input file

## SYNOPSIS
**podman kube** *subcommand*

## DESCRIPTION
The kube command will recreate containers, pods or volumes based on the input from a structured (like YAML)
file input.  Containers will be automatically started.

## COMMANDS

| Command  | Man Page                                             | Description                                                                   |
| -------  | ---------------------------------------------------- | ----------------------------------------------------------------------------- |
| down     | [podman-kube-down(1)](podman-kube-down.1.md)         | Remove containers and pods based on Kubernetes YAML.                          |
| generate | [podman-kube-generate(1)](podman-kube-generate.1.md) | Generate Kubernetes YAML based on containers, pods or volumes.                |
| play     | [podman-kube-play(1)](podman-kube-play.1.md)         | Create containers, pods and volumes based on Kubernetes YAML.                 |

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-pod(1)](podman-pod.1.md)**, **[podman-container(1)](podman-container.1.md)**, **[podman-kube-play(1)](podman-kube-play.1.md)**, **[podman-kube-down(1)](podman-kube-down.1.md)**, **[podman-kube-generate(1)](podman-kube-generate.1.md)**

## HISTORY
December 2018, Originally compiled by Brent Baude (bbaude at redhat dot com)
