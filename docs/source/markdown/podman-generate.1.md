% podman-generate(1)

## NAME
podman\-generate - Generate structured data based on containers, pods or volumes

## SYNOPSIS
**podman generate** *subcommand*

## DESCRIPTION
The generate command will create structured output (like YAML) based on a container, pod or volume.

## COMMANDS

| Command | Man Page                                                   | Description                                                                         |
|---------|------------------------------------------------------------|-------------------------------------------------------------------------------------|
| kube    | [podman-generate-kube(1)](podman-generate-kube.1.md)       | Generate Kubernetes YAML based on containers, pods or volumes.                               |
| systemd | [podman-generate-systemd(1)](podman-generate-systemd.1.md) | Generate systemd unit file(s) for a container or pod.                               |


## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-pod(1)](podman-pod.1.md)**, **[podman-container(1)](podman-container.1.md)**
